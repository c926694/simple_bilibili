package danmaku

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/snowflake"
	"simple_bilibili/backend/internal/ws"
)

type Service struct {
	videos       *video.Service
	store        *Store
	hub          *ws.Hub
	publisher    *rocketmq.Producer
	ids          *snowflake.Generator
	logger       *slog.Logger
	segmentLoads singleflight.Group
	flushMu      sync.Mutex
}

func NewService(videos *video.Service, store *Store, hub *ws.Hub, publisher *rocketmq.Producer, ids *snowflake.Generator, logger *slog.Logger) *Service {
	return &Service{videos: videos, store: store, hub: hub, publisher: publisher, ids: ids, logger: logger}
}

func (s *Service) Send(ctx context.Context, userID int64, bvid string, input sendInput) (Message, error) {
	info, err := s.videos.PublishedVideoPart(ctx, bvid, input.PartID)
	if err != nil {
		return Message{}, err
	}
	if input.TimeOffsetMS > max(int64(info.DurationSec)*1000+2000, 1000) {
		return Message{}, errcode.InvalidParams
	}
	item := Message{ID: s.ids.Next(), VideoID: info.VideoID, PartID: info.PartID, UserID: userID,
		Content: input.Content, TimeOffsetMS: input.TimeOffsetMS, Mode: input.Mode, FontSize: input.FontSize, Color: input.Color,
		CreatedAt: time.Now().UTC().Truncate(time.Second)}
	payload, err := json.Marshal(item)
	if err != nil {
		return Message{}, err
	}
	if err := s.hub.BroadcastVideo(item.VideoID, "danmaku", item); err != nil {
		return Message{}, err
	}
	if _, err := s.publisher.PublishJSON(ctx, mq.DanmakuPersistTopic, item, rocketmq.WithKeys(strconv.FormatInt(item.ID, 10))); err != nil {
		return Message{}, err
	}
	// 热数据与数据库段分开保存，发布后立即可查。
	if err := s.store.addHot(ctx, item, string(payload)); err != nil {
		return Message{}, fmt.Errorf("写入刚发送的弹幕: %w", err)
	}
	return item, nil
}

func (s *Service) History(ctx context.Context, bvid string, partID, segment int64) (historyResponse, error) {
	info, err := s.videos.PublishedVideoPart(ctx, bvid, partID)
	if err != nil {
		return historyResponse{}, err
	}
	result := historyResponse{Segment: segment, SegmentDurationMS: segmentDurationMS, OnlineCount: s.hub.OnlineCount(info.VideoID), Items: []Message{}}
	if segment*segmentDurationMS > max(int64(info.DurationSec)*1000+2000, 1000) {
		return result, nil
	}
	baseKey, hotKey := segmentKeys(partID, segment)
	// 合并同一分段的数据库加载，再读取当前热数据。
	load := s.segmentLoads.DoChan(baseKey, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.loadSegment(loadCtx, baseKey, info.VideoID, partID, segment)
	})
	var items []Message
	select {
	case <-ctx.Done():
		return historyResponse{}, ctx.Err()
	case saved := <-load:
		if saved.Err != nil {
			return historyResponse{}, saved.Err
		}
		items = saved.Val.([]Message)
	}
	hot, err := s.store.redis.ZRangeByScore(ctx, hotKey, &redis.ZRangeBy{
		Min: fmt.Sprint(time.Now().Add(-hotRetention).UnixMilli()), Max: "+inf",
	}).Result()
	if err != nil {
		return historyResponse{}, err
	}
	// 以弹幕 ID 去重，历史段与热数据使用相同的排序规则。
	result.Items, err = mergeSegment(items, hot)
	if err != nil {
		return historyResponse{}, err
	}
	return result, nil
}

func (s *Service) loadSegment(ctx context.Context, key string, videoID, partID, segment int64) ([]Message, error) {
	data, err := s.store.redis.Get(ctx, key).Bytes()
	if err == nil {
		var items []Message
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("读取弹幕历史缓存: %w", err)
		}
		return items, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}
	items, err := s.store.segment(ctx, videoID, partID, segment)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	ttl := 2*time.Minute + time.Duration(rand.IntN(60))*time.Second
	if len(items) == 0 {
		ttl = 20*time.Second + time.Duration(rand.IntN(10))*time.Second
	}
	if err := s.store.redis.Set(ctx, key, payload, ttl).Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func mergeSegment(base []Message, hot []string) ([]Message, error) {
	byID := make(map[int64]Message, len(base)+len(hot))
	for _, item := range base {
		byID[item.ID] = item
	}
	for _, data := range hot {
		var item Message
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			return nil, fmt.Errorf("读取热弹幕: %w", err)
		}
		byID[item.ID] = item
	}
	combined := make([]Message, 0, len(byID))
	for _, item := range byID {
		combined = append(combined, item)
	}
	sort.Slice(combined, func(i, j int) bool {
		if combined[i].TimeOffsetMS == combined[j].TimeOffsetMS {
			return combined[i].ID < combined[j].ID
		}
		return combined[i].TimeOffsetMS < combined[j].TimeOffsetMS
	})
	return combined, nil
}
