package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/internal/infra/ffmpeg"
	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
)

type Consumer struct {
	service  *Service
	ffmpeg   *ffmpeg.Client
	config   config.FFmpeg
	client   *rocketmq.PushConsumer
	ctx      context.Context
	cancel   context.CancelFunc
	slots    chan struct{}
	mu       sync.Mutex
	closing  bool
	inFlight sync.WaitGroup
}

func (s *Service) StartConsumer(ctx context.Context, messaging config.RocketMQ, cfg config.FFmpeg) (*Consumer, error) {
	if err := os.MkdirAll(cfg.WorkDir, 0o750); err != nil {
		return nil, fmt.Errorf("创建转码工作目录: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	client, err := rocketmq.NewPushConsumer(ctx, messaging, mq.VideoTranscodeConsumerGroup,
		rocketmq.WithConsumeFromWhere(rocketmq.ConsumeFromFirstOffset),
		// 等待信号量与执行任务分别受根上下文和 FFmpeg 执行期限控制。
		rocketmq.WithHandleTimeout(time.Duration(1<<63-1)))
	if err != nil {
		cancel()
		return nil, err
	}
	consumer := &Consumer{service: s, ffmpeg: ffmpeg.New(cfg), config: cfg, client: client, ctx: ctx, cancel: cancel, slots: make(chan struct{}, cfg.MaxConcurrent)}
	if err := client.Subscribe(mq.VideoTranscodeTopic, "*", consumer.handle); err != nil {
		cancel()
		return nil, errors.Join(err, client.Close())
	}
	if err := client.Start(); err != nil {
		cancel()
		return nil, errors.Join(err, client.Close())
	}
	consumer.inFlight.Add(1)
	go consumer.dispatchLoop()
	return consumer, nil
}

func (c *Consumer) Close() error {
	c.mu.Lock()
	c.closing = true
	c.cancel()
	c.mu.Unlock()
	err := c.client.Close()
	c.inFlight.Wait()
	return err
}

func (c *Consumer) dispatchLoop() {
	defer c.inFlight.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for c.ctx.Err() == nil {
		if err := c.service.dispatchDue(c.ctx); err != nil && c.ctx.Err() == nil {
			c.service.logger.ErrorContext(c.ctx, "安排待执行的转码任务", "error", err)
		}
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Consumer) handle(messageCtx context.Context, msg rocketmq.Message) error {
	c.mu.Lock()
	if c.closing {
		c.mu.Unlock()
		return context.Canceled
	}
	c.inFlight.Add(1)
	c.mu.Unlock()
	defer c.inFlight.Done()

	var event mq.VideoTranscodeTask
	if err := msg.DecodeJSON(&event); err != nil {
		return rocketmq.PermanentError(err)
	}
	if event.VideoID <= 0 || event.PartID <= 0 || event.AuthorID <= 0 || event.JobID <= 0 || event.Attempt < 0 || event.RawObjectKey == "" {
		return rocketmq.PermanentError(errors.New("无效的稿件转码消息"))
	}
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
	root := rocketmq.WithTraceID(c.ctx, rocketmq.TraceIDFromContext(messageCtx))
	ctx, cancel := context.WithTimeout(root, c.config.TaskTimeout)
	defer cancel()
	// 领取分 P 后才启动 FFmpeg，重复消息会被忽略。
	row, p, err := c.service.claimJob(ctx, event, c.config.TaskTimeout)
	if err != nil || p.ID == 0 {
		return err
	}
	workErr := c.service.invalidate(ctx, row.BVID)
	if workErr == nil {
		// 下载、读取视频信息、生成封面与 HLS。
		p, workErr = c.transcode(ctx, row, p)
	}
	permanent := errors.Is(workErr, ffmpeg.ErrInvalidMedia) || errors.Is(workErr, minio.ErrObjectNotFound)
	canceled := workErr != nil && root.Err() != nil
	delays := [...]time.Duration{10 * time.Second, 30 * time.Second, 2 * time.Minute}
	var delay time.Duration
	if workErr != nil && !canceled && !permanent && p.Attempt < len(delays) {
		delay = delays[p.Attempt]
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(root), 5*time.Second)
	defer finishCancel()
	// 退出与超时后仍尝试提交分 P 状态，执行期限也允许下次启动恢复。
	if err := c.service.completeJob(finishCtx, row, p, workErr, delay, canceled); err != nil {
		return errors.Join(workErr, err)
	}
	if workErr != nil && !canceled && delay == 0 {
		return rocketmq.PermanentError(workErr)
	}
	return nil
}

func (c *Consumer) transcode(ctx context.Context, row video, p part) (result part, err error) {
	dir, err := os.MkdirTemp(c.config.WorkDir, "part-"+fmt.Sprint(p.ID)+"-")
	if err != nil {
		return p, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	input := filepath.Join(dir, "source")
	if err := c.service.objects.DownloadFile(ctx, p.RawKey, input); err != nil {
		return p, err
	}
	meta, err := c.ffmpeg.Probe(ctx, input)
	if err != nil {
		return p, err
	}
	// 作者没有指定封面时，使用视频画面并按条件回写。
	if row.CoverURL == "" {
		if err := c.automaticCover(ctx, dir, input, row, meta.DurationSec); err != nil {
			return p, err
		}
	}
	qualities, err := c.ffmpeg.TranscodeHLS(ctx, input, dir, meta, c.config.Threads)
	if err != nil {
		return p, err
	}
	// 分片先上传，播放列表最后公开。
	for _, quality := range qualities {
		url, err := c.uploadQuality(ctx, dir, p, quality.Name)
		if err != nil {
			return p, err
		}
		switch quality.Name {
		case "1080p":
			p.HLS1080 = &url
		case "720p":
			p.HLS720 = &url
		case "480p":
			p.HLS480 = &url
		}
	}
	p.Duration = meta.DurationSec
	return p, nil
}

func (c *Consumer) automaticCover(ctx context.Context, dir, input string, row video, duration int) error {
	local := filepath.Join(dir, "cover.jpg")
	if err := c.ffmpeg.ExtractCover(ctx, input, local, duration); err != nil {
		return err
	}
	key := fmt.Sprintf("covers/%d/%d_auto.jpg", row.AuthorID, row.ID)
	if err := c.putFile(ctx, local, key, "image/jpeg"); err != nil {
		return err
	}
	if err := c.service.store.setAutomaticCover(ctx, row.ID, "/storage/"+key); err != nil {
		return err
	}
	return c.service.invalidate(ctx, row.BVID)
}

func (c *Consumer) uploadQuality(ctx context.Context, dir string, p part, quality string) (string, error) {
	local := filepath.Join(dir, quality)
	entries, err := os.ReadDir(local)
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("hls/%d/%d/%s/", p.VideoID, p.ID, quality)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".ts" {
			continue
		}
		if err := c.putFile(ctx, filepath.Join(local, entry.Name()), prefix+entry.Name(), "video/mp2t"); err != nil {
			return "", err
		}
	}
	key := prefix + "index.m3u8"
	if err := c.putFile(ctx, filepath.Join(local, "index.m3u8"), key, "application/vnd.apple.mpegurl"); err != nil {
		return "", err
	}
	return "/storage/" + key, nil
}

func (c *Consumer) putFile(ctx context.Context, local, object, contentType string) (err error) {
	file, err := os.Open(local)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return c.service.objects.PutObject(ctx, object, file, info.Size(), contentType)
}
