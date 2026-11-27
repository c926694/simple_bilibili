package rocketmq

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/rlog"

	"simple_bilibili/backend/internal/config"
)

type PullBatchHandler func(ctx context.Context, batch []Message) error

type PullBatchConfig struct {
	BatchSize    int
	PollTimeout  time.Duration
	RetryBackoff time.Duration
}

type pullSettings struct {
	fromWhere ConsumeFromWhere
}

type PullOption func(*pullSettings)

func WithPullConsumeFromWhere(from ConsumeFromWhere) PullOption {
	return func(settings *pullSettings) { settings.fromWhere = from }
}

type PullConsumer struct {
	options  []consumer.Option
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
	closed   bool
	closeErr error
}

func NewPullConsumer(ctx context.Context, cfg config.RocketMQ, groupName string, opts ...PullOption) (*PullConsumer, error) {
	settings := pullSettings{fromWhere: ConsumeFromLastOffset}
	for _, opt := range opts {
		opt(&settings)
	}
	if strings.TrimSpace(groupName) == "" || settings.fromWhere < ConsumeFromLastOffset || settings.fromWhere > ConsumeFromTimestamp {
		return nil, errors.New("RocketMQ 拉消费者组名或起始位点设置无效")
	}
	addresses, err := resolveNameServers(ctx, cfg.NameServers)
	if err != nil {
		return nil, err
	}
	return &PullConsumer{options: []consumer.Option{
		consumer.WithNameServer(addresses),
		consumer.WithGroupName(groupName),
		consumer.WithInstance("pull-" + rand.Text()),
		consumer.WithConsumeFromWhere(settings.fromWhere),
		consumer.WithAutoCommit(false),
	}}, nil
}

func (c *PullConsumer) RunBatch(ctx context.Context, topic, tagExpr string, cfg PullBatchConfig, handler PullBatchHandler) (err error) {
	if strings.TrimSpace(topic) == "" || handler == nil || cfg.BatchSize <= 0 || cfg.BatchSize > math.MaxInt32 ||
		cfg.PollTimeout <= 0 || cfg.RetryBackoff <= 0 {
		return errors.New("RocketMQ 拉取 Topic、批次大小、处理函数、等待时间或重试间隔无效")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.mu.Lock()
	if c.closed || c.done != nil {
		c.mu.Unlock()
		return errors.New("RocketMQ 拉消费者已运行或已关闭")
	}
	c.cancel, c.done = cancel, make(chan struct{})
	c.mu.Unlock()
	defer close(c.done)

	opts := append(c.options, consumer.WithPullBatchSize(int32(cfg.BatchSize)))
	client, err := rocketmq.NewPullConsumer(opts...)
	if err != nil {
		return fmt.Errorf("初始化 RocketMQ 拉消费者: %w", err)
	}
	selector := consumer.MessageSelector{Type: consumer.TAG, Expression: subscription(tagExpr)}
	if err := client.Subscribe(topic, selector); err != nil {
		return fmt.Errorf("订阅 RocketMQ Topic %q: %w", topic, err)
	}
	if err := client.Start(); err != nil {
		return fmt.Errorf("启动 RocketMQ 拉消费者: %w", err)
	}
	defer func() {
		c.closeErr = client.Shutdown()
		err = errors.Join(err, c.closeErr)
	}()

	for ctx.Err() == nil {
		request, err := client.Poll(ctx, cfg.PollTimeout)
		if errors.Is(err, consumer.ErrNoNewMsg) {
			continue
		}
		if err != nil {
			return fmt.Errorf("批量拉取 RocketMQ 消息: %w", err)
		}
		batch := make([]Message, 0, len(request.GetMsgList()))
		for _, message := range request.GetMsgList() {
			batch = append(batch, fromSDK(message))
		}
		handled, err := handleBatch(ctx, cfg.RetryBackoff, handler, batch)
		if err != nil || !handled {
			return err
		}
		// 成功才更新 SDK 位点并持久化，取消信号不会跳过已完成批次的确认。
		commitCtx := context.WithoutCancel(ctx)
		client.ACK(commitCtx, request, consumer.ConsumeSuccess)
		if err := client.PersistOffset(commitCtx, topic); err != nil {
			return fmt.Errorf("提交 RocketMQ 消费位点: %w", err)
		}
	}
	return nil
}

func handleBatch(ctx context.Context, backoff time.Duration, handler PullBatchHandler, batch []Message) (bool, error) {
	// 同一批次只在消息共用追踪标识时绑定到批次上下文。
	traceID := batch[0].Properties[TraceIDProperty]
	for _, msg := range batch[1:] {
		if msg.Properties[TraceIDProperty] != traceID {
			traceID = ""
			break
		}
	}
	ctx = WithTraceID(ctx, traceID)
	for ctx.Err() == nil {
		err := handler(ctx, batch)
		if err == nil {
			return true, nil
		}
		if IsPermanentError(err) {
			return false, err
		}
		rlog.Warning("批量处理 RocketMQ 消息失败，等待重试", map[string]any{
			"topic": batch[0].Topic, "count": len(batch), "error": err.Error(),
		})
		// 重试同一批次，期间不给 SDK 发送确认。
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, nil
		case <-timer.C:
		}
	}
	return false, nil
}

func (c *PullConsumer) Close() error {
	c.mu.Lock()
	c.closed = true
	cancel, done := c.cancel, c.done
	c.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	<-done
	return c.closeErr
}
