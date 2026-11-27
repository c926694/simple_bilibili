package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/tx"
)

type hotHistory struct {
	mq.WatchHistoryEvent
	Micros int64 `json:"watched_at_us"`
}

var storeProgress = redis.NewScript(`
local clock=tonumber(ARGV[2])
local all=tonumber(redis.call('HGET',KEYS[2],'all') or 0)
local one=tonumber(redis.call('HGET',KEYS[2],ARGV[1]) or 0)
if clock <= math.max(all,one) then return 0 end
local previous=redis.call('HGET',KEYS[1],ARGV[1])
if previous then
	local row=cjson.decode(previous)
	local newID=ARGV[4]
	local oldID=row.id
	if row.watched_at_us > clock or (row.watched_at_us == clock and (#oldID > #newID or (#oldID == #newID and oldID >= newID))) then return 1 end
end
redis.call('HSET',KEYS[1],ARGV[1],ARGV[3])
redis.call('EXPIRE',KEYS[1],86400)
return 1
`)

var deleteProgress = redis.NewScript(`
local field=ARGV[1]
local cutoff=tonumber(ARGV[2])
local previous=tonumber(redis.call('HGET',KEYS[2],field) or 0)
if cutoff > previous then redis.call('HSET',KEYS[2],field,ARGV[2]) end
if field == 'all' then
	local entries=redis.call('HGETALL',KEYS[1])
	for i=1,#entries,2 do
		if cjson.decode(entries[i+1]).watched_at_us <= cutoff then redis.call('HDEL',KEYS[1],entries[i]) end
	end
else
	local entry=redis.call('HGET',KEYS[1],field)
	if entry and cjson.decode(entry).watched_at_us <= cutoff then redis.call('HDEL',KEYS[1],field) end
end
return 0
`)

func historyKeys(userID int64) (string, string) {
	prefix := fmt.Sprintf("watch:{%d}", userID)
	return prefix + ":hot", prefix + ":deleted"
}

func (s *Store) saveHotProgress(ctx context.Context, row mq.WatchHistoryEvent) (bool, error) {
	hot, deletes := historyKeys(row.UserID)
	payload, err := json.Marshal(hotHistory{row, row.WatchedAt.UnixMicro()})
	if err != nil {
		return false, err
	}
	result, err := storeProgress.Run(ctx, s.redis, []string{hot, deletes}, row.VideoID, row.WatchedAt.UnixMicro(), string(payload), row.ID).Int()
	return result == 1, err
}

func (s *Store) cutoffs(ctx context.Context, userID int64) (map[string]int64, error) {
	_, key := historyKeys(userID)
	values, err := s.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(values))
	for field, value := range values {
		stamp, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		result[field] = stamp
	}
	return result, nil
}

func (s *Store) historyRows(ctx context.Context, userID, videoID int64) ([]mq.WatchHistoryEvent, error) {
	query := "SELECT id, user_id, video_id, part_id, progress_sec, watched_at FROM watch_history WHERE user_id = ?"
	args := []any{userID}
	if videoID != 0 {
		query += " AND video_id = ?"
		args = append(args, videoID)
	}
	var rows []mq.WatchHistoryEvent
	err := s.db.SelectContext(ctx, &rows, query, args...)
	return rows, err
}

func (s *Store) hotRows(ctx context.Context, userID, videoID int64) ([]mq.WatchHistoryEvent, error) {
	key, _ := historyKeys(userID)
	var entries []string
	if videoID == 0 {
		var err error
		entries, err = s.redis.HVals(ctx, key).Result()
		if err != nil {
			return nil, err
		}
	} else {
		data, err := s.redis.HGet(ctx, key, strconv.FormatInt(videoID, 10)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		if err == nil {
			entries = []string{data}
		}
	}
	rows := make([]mq.WatchHistoryEvent, 0, len(entries))
	for _, value := range entries {
		var row hotHistory
		if err := json.Unmarshal([]byte(value), &row); err != nil {
			return nil, fmt.Errorf("读取观看历史热缓存: %w", err)
		}
		rows = append(rows, row.WatchHistoryEvent)
	}
	return rows, nil
}

func (s *Store) removeHistory(ctx context.Context, userID, videoID int64, cutoff time.Time) error {
	query := "DELETE FROM watch_history WHERE user_id = ? AND watched_at <= ?"
	args := []any{userID, cutoff}
	field := "all"
	if videoID != 0 {
		query += " AND video_id = ?"
		args = append(args, videoID)
		field = strconv.FormatInt(videoID, 10)
	}
	if _, err := tx.Executor(ctx, s.db).ExecContext(ctx, query, args...); err != nil {
		return err
	}
	hot, deletes := historyKeys(userID)
	return deleteProgress.Run(ctx, s.redis, []string{hot, deletes}, field, cutoff.UnixMicro()).Err()
}

func (s *Store) upsertHistory(ctx context.Context, rows []mq.WatchHistoryEvent) error {
	if len(rows) == 0 {
		return nil
	}
	var query strings.Builder
	query.WriteString("INSERT INTO watch_history (id,user_id,video_id,part_id,progress_sec,watched_at) VALUES ")
	args := make([]any, 0, len(rows)*6)
	for i, row := range rows {
		if i != 0 {
			query.WriteByte(',')
		}
		query.WriteString("(?,?,?,?,?,?)")
		args = append(args, row.ID, row.UserID, row.VideoID, row.PartID, row.Progress, row.WatchedAt)
	}
	// watched_at 与 ID 最后更新，前面的字段都按原记录判断新旧。
	const newer = "(incoming.watched_at > watch_history.watched_at OR (incoming.watched_at = watch_history.watched_at AND incoming.id > watch_history.id))"
	query.WriteString(" AS incoming ON DUPLICATE KEY UPDATE ")
	for _, field := range []string{"part_id", "progress_sec", "id"} {
		query.WriteString(field + " = IF(" + newer + ", incoming." + field + ", watch_history." + field + "), ")
	}
	query.WriteString("watched_at = GREATEST(watch_history.watched_at, incoming.watched_at)")
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, query.String(), args...)
	return err
}
