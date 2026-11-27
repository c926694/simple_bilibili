package interaction

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/tx"
)

type progressResponse struct {
	VideoID   int64     `json:"video_id,string"`
	PartID    int64     `json:"part_id,string"`
	Progress  int       `json:"progress_sec"`
	WatchedAt time.Time `json:"watched_at"`
}

type historyItem struct {
	Video video.Summary `json:"video"`
	watchProgress
	PartTitle string `json:"part_title"`
}

type historyPage struct {
	Items []historyItem `json:"items"`
	Total int           `json:"total"`
	page
}

func (s *Service) Progress(ctx context.Context, userID int64, bvid string, partID int64, seconds int) (progressResponse, error) {
	part, err := s.videos.PublishedVideoPart(ctx, bvid, partID)
	if err != nil {
		return progressResponse{}, err
	}
	event := mq.WatchHistoryEvent{ID: s.ids.Next(), UserID: userID, VideoID: part.VideoID, PartID: part.PartID,
		Progress: min(seconds, part.DurationSec), WatchedAt: time.Now().UTC().Truncate(time.Microsecond)}
	updated, err := s.store.saveHotProgress(ctx, event)
	if err != nil {
		return progressResponse{}, err
	}
	if !updated {
		return progressResponse{}, errcode.InvalidParams
	}
	// 热进度即时可见，批量消费负责保存历史。
	if _, err := s.publisher.PublishJSON(ctx, mq.WatchHistorySyncTopic, event, rocketmq.WithKeys(strconv.FormatInt(event.ID, 10))); err != nil {
		return progressResponse{}, err
	}
	return progressResponse{event.VideoID, event.PartID, event.Progress, event.WatchedAt}, nil
}

func (s *Service) History(ctx context.Context, userID int64, keyword string, p page) (historyPage, error) {
	rows, err := s.mergedHistory(ctx, userID, 0)
	if err != nil {
		return historyPage{}, err
	}
	// 合并新旧进度后批量读取可见视频与分 P。
	videos := make([]int64, 0, len(rows))
	parts := make([]int64, 0, len(rows))
	for _, row := range rows {
		videos = append(videos, row.VideoID)
		parts = append(parts, row.PartID)
	}
	videoInfo, err := s.videos.SummariesByIDs(ctx, videos)
	if err != nil {
		return historyPage{}, err
	}
	partInfo, err := s.videos.PartSummariesByIDs(ctx, parts)
	if err != nil {
		return historyPage{}, err
	}
	items := make([]historyItem, 0, len(rows))
	keyword = strings.ToLower(keyword)
	for _, row := range rows {
		summary, exists := videoInfo[row.VideoID]
		if !exists || (keyword != "" && !strings.Contains(strings.ToLower(summary.Title), keyword)) {
			continue
		}
		part, exists := partInfo[row.PartID]
		if !exists || part.VideoID != row.VideoID {
			continue
		}
		items = append(items, historyItem{Video: summary, PartTitle: part.Title, watchProgress: watchProgress{
			PartID: row.PartID, PartIndex: part.PartIndex, Progress: row.Progress, WatchedAt: row.WatchedAt}})
	}
	start, end := p.bounds(len(items))
	return historyPage{Items: items[start:end], Total: len(items), page: p}, nil
}

func (s *Service) progressForVideo(ctx context.Context, userID, videoID int64) (*watchProgress, error) {
	rows, err := s.mergedHistory(ctx, userID, videoID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	info, err := s.videos.PartSummariesByIDs(ctx, []int64{row.PartID})
	if err != nil {
		return nil, err
	}
	part, ok := info[row.PartID]
	if !ok || part.VideoID != videoID {
		return nil, nil
	}
	return &watchProgress{PartID: row.PartID, PartIndex: part.PartIndex, Progress: row.Progress, WatchedAt: row.WatchedAt}, nil
}

func (s *Service) DeleteHistory(ctx context.Context, userID int64, bvid string) error {
	var videoID int64
	if bvid != "" {
		published, err := s.videos.PublishedVideo(ctx, bvid)
		if err != nil {
			return err
		}
		videoID = published.ID
	}
	cutoff := time.Now().UTC().Truncate(time.Microsecond)
	return s.userTx(ctx, userID, func(ctx context.Context) error {
		// 同一用户的删除与批量回写共用用户行锁。
		return s.store.removeHistory(ctx, userID, videoID, cutoff)
	})
}

func (s *Service) mergedHistory(ctx context.Context, userID, videoID int64) ([]mq.WatchHistoryEvent, error) {
	rows, err := s.store.historyRows(ctx, userID, videoID)
	if err != nil {
		return nil, err
	}
	hot, err := s.store.hotRows(ctx, userID, videoID)
	if err != nil {
		return nil, err
	}
	deleted, err := s.store.cutoffs(ctx, userID)
	if err != nil {
		return nil, err
	}
	latest := make(map[int64]mq.WatchHistoryEvent, len(rows)+len(hot))
	for _, row := range append(rows, hot...) {
		if !afterDelete(row, deleted) {
			continue
		}
		if old, ok := latest[row.VideoID]; !ok || newerProgress(row, old) {
			latest[row.VideoID] = row
		}
	}
	result := make([]mq.WatchHistoryEvent, 0, len(latest))
	for _, row := range latest {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return newerProgress(result[i], result[j]) })
	return result, nil
}

func newerProgress(left, right mq.WatchHistoryEvent) bool {
	if left.WatchedAt.Equal(right.WatchedAt) {
		return left.ID > right.ID
	}
	return left.WatchedAt.After(right.WatchedAt)
}

func afterDelete(row mq.WatchHistoryEvent, marks map[string]int64) bool {
	return row.WatchedAt.UnixMicro() > max(marks["all"], marks[strconv.FormatInt(row.VideoID, 10)])
}

func (s *Service) persistHistory(ctx context.Context, batch []rocketmq.Message) error {
	latest := make(map[[2]int64]mq.WatchHistoryEvent)
	for _, msg := range batch {
		var row mq.WatchHistoryEvent
		if err := msg.DecodeJSON(&row); err != nil || row.ID <= 0 || row.UserID <= 0 || row.VideoID <= 0 ||
			row.PartID <= 0 || row.Progress < 0 || row.WatchedAt.IsZero() {
			s.logger.WarnContext(ctx, "跳过无效的观看历史事件", "msg_id", msg.MsgID, "error", err)
			continue
		}
		key := [2]int64{row.UserID, row.VideoID}
		if prior, exists := latest[key]; !exists || newerProgress(row, prior) {
			latest[key] = row
		}
	}
	ordered := make([]mq.WatchHistoryEvent, 0, len(latest))
	for _, row := range latest {
		ordered = append(ordered, row)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].UserID == ordered[j].UserID {
			return ordered[i].VideoID < ordered[j].VideoID
		}
		return ordered[i].UserID < ordered[j].UserID
	})
	return tx.WithTx(ctx, s.store.db, func(ctx context.Context) error {
		var currentUser int64
		var marks map[string]int64
		rows := make([]mq.WatchHistoryEvent, 0, len(ordered))
		for _, row := range ordered {
			if currentUser != row.UserID {
				currentUser = row.UserID
				// 按用户 ID 顺序加锁，再读取删除时间。
				if err := s.users.LockForUpdate(ctx, currentUser); err != nil {
					if err == errcode.UserNotFound {
						marks = nil
						continue
					}
					return err
				}
				var err error
				marks, err = s.store.cutoffs(ctx, currentUser)
				if err != nil {
					return err
				}
			}
			if marks != nil && afterDelete(row, marks) {
				rows = append(rows, row)
			}
		}
		return s.store.upsertHistory(ctx, rows)
	})
}
