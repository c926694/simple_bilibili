package interaction

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/mq"
)

const (
	playBuffer     = "interaction:{play}:buffer"
	playPending    = "interaction:{play}:pending"
	playGeneration = "interaction:{play}:generation"
	playEvents     = "interaction:{play}:events"
)

var countPlay = redis.NewScript(`
if not redis.call('SET',KEYS[1],'1','NX','EX',ARGV[1]) then return 0 end
redis.call('HINCRBY',KEYS[2],ARGV[2],1)
return 1
`)

var releaseShare = redis.NewScript(`
if redis.call('GET',KEYS[1]) == ARGV[1] then return redis.call('DEL',KEYS[1]) end
return 0
`)

var takePlays = redis.NewScript(`
if redis.call('EXISTS',KEYS[2]) == 0 then
	if redis.call('EXISTS',KEYS[1]) == 0 then return {} end
	redis.call('RENAME',KEYS[1],KEYS[2])
	redis.call('SET',KEYS[3],ARGV[1])
end
local values=redis.call('HGETALL',KEYS[2])
table.insert(values,1,redis.call('GET',KEYS[3]))
return values
`)

var claimPlayEvent = redis.NewScript(`
if redis.call('GET',KEYS[1]) ~= ARGV[1] or redis.call('HEXISTS',KEYS[2],ARGV[2]) == 0 then return '' end
redis.call('HSETNX',KEYS[3],ARGV[2],ARGV[3])
return redis.call('HGET',KEYS[3],ARGV[2])
`)

var ackPlayEvent = redis.NewScript(`
if redis.call('GET',KEYS[1]) == ARGV[1] and redis.call('HGET',KEYS[3],ARGV[2]) == ARGV[3] then
	redis.call('HDEL',KEYS[2],ARGV[2])
	redis.call('HDEL',KEYS[3],ARGV[2])
	if redis.call('EXISTS',KEYS[2]) == 0 then redis.call('DEL',KEYS[1]) end
end
return 0
`)

func viewerKey(userID int64, ip string) string {
	if userID != 0 {
		return "user:" + strconv.FormatInt(userID, 10)
	}
	return "ip:" + ip
}

func (s *Service) Play(ctx context.Context, userID int64, ip, bvid string) (countedResponse, error) {
	published, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return countedResponse{}, err
	}
	id := strconv.FormatInt(published.ID, 10)
	key := "interaction:{play}:seen:" + id + ":" + viewerKey(userID, ip)
	counted, err := countPlay.Run(ctx, s.store.redis, []string{key, playBuffer}, max(published.DurationSec*3, 30), id).Int()
	return countedResponse{Counted: counted == 1}, err
}

func (s *Service) Share(ctx context.Context, userID int64, ip, bvid string) (countedResponse, error) {
	published, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return countedResponse{}, err
	}
	id := s.ids.Next()
	key := fmt.Sprintf("interaction:share:%d:%s", published.ID, viewerKey(userID, ip))
	ok, err := s.store.redis.SetNX(ctx, key, id, time.Minute).Result()
	if err != nil || !ok {
		return countedResponse{}, err
	}
	if err := s.store.saveDelta(ctx, mq.VideoStatDelta{EventID: id, VideoID: published.ID, ShareDelta: 1}); err != nil {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		return countedResponse{}, errors.Join(err, releaseShare.Run(releaseCtx, s.store.redis, []string{key}, id).Err())
	}
	s.wakeOutbox()
	return countedResponse{Counted: true}, nil
}

func (s *Service) flushPlays(ctx context.Context) error {
	cache := s.store.redis
	batch, err := takePlays.Run(ctx, cache, []string{playBuffer, playPending, playGeneration}, s.ids.Next()).StringSlice()
	if err != nil || len(batch) == 0 {
		return err
	}
	keys := []string{playGeneration, playPending, playEvents}
	for i := 1; i < len(batch); i += 2 {
		videoKey := batch[i]
		videoID, err := strconv.ParseInt(videoKey, 10, 64)
		if err != nil {
			return err
		}
		delta, err := strconv.ParseInt(batch[i+1], 10, 64)
		if err != nil {
			return err
		}
		// Redis 保留待发增量及其事件 ID，失败后使用原 ID 重投。
		id, err := claimPlayEvent.Run(ctx, cache, keys, batch[0], videoKey, s.ids.Next()).Text()
		if err != nil {
			return err
		}
		if id == "" {
			continue
		}
		eventID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return err
		}
		if err := s.publishDelta(ctx, mq.VideoStatDelta{EventID: eventID, VideoID: videoID, PlayDelta: delta}); err != nil {
			return err
		}
		if err := ackPlayEvent.Run(ctx, cache, keys, batch[0], videoKey, id).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) publishDelta(ctx context.Context, event mq.VideoStatDelta) error {
	_, err := s.publisher.PublishJSON(ctx, mq.VideoStatSyncTopic, event, rocketmq.WithKeys(strconv.FormatInt(event.EventID, 10)))
	return err
}
