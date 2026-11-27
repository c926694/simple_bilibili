package video

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/errcode"
)

func (s *Service) Categories(ctx context.Context) ([]categoryListing, error) {
	return s.store.categories(ctx)
}

func (s *Service) List(ctx context.Context, filter listFilter) (videoPage, error) {
	var rows []video
	var total int64
	var err error
	if filter.Sort == "latest" {
		rows, total, err = s.store.latest(ctx, filter)
	} else {
		// 先筛选视频 ID，再批量读取统计并按计数排序。
		rows, total, err = s.rankedVideos(ctx, filter)
	}
	if err != nil {
		return videoPage{}, err
	}
	// 当前页关联的作者、分区、统计与选集分别批量查询。
	items, err := s.buildItems(ctx, rows, filter.Mine)
	if err != nil {
		return videoPage{}, err
	}
	return videoPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.Size}, nil
}

func (s *Service) Search(ctx context.Context, filter listFilter) (searchResult, error) {
	videos, err := s.List(ctx, filter)
	if err != nil {
		return searchResult{}, err
	}
	users, total, err := s.users.SearchPublicUsers(ctx, filter.Keyword, filter.offset(), filter.Size)
	if err != nil {
		return searchResult{}, err
	}
	return searchResult{Videos: videos, Users: userPage{Items: users, Total: total}}, nil
}

func (s *Service) rankedVideos(ctx context.Context, f listFilter) ([]video, int64, error) {
	ids, err := s.store.matchingIDs(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(ids))
	if f.offset() >= total {
		return []video{}, total, nil
	}
	stats := make(map[int64]statistic, len(ids))
	for start := 0; start < len(ids); start += 1000 {
		rows, err := s.store.statsByIDs(ctx, ids[start:min(start+1000, len(ids))])
		if err != nil {
			return nil, 0, err
		}
		for _, row := range rows {
			stats[row.VideoID] = row
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := stats[ids[i]], stats[ids[j]]
		var order int
		switch f.Sort {
		case "play":
			order = cmp.Compare(left.PlayCount, right.PlayCount)
		case "danmaku":
			order = cmp.Compare(left.DanmakuCount, right.DanmakuCount)
		default:
			order = cmp.Compare(left.HotScore, right.HotScore)
		}
		if order == 0 {
			return ids[i] > ids[j]
		}
		return order > 0
	})
	window := ids[f.offset():min(f.offset()+int64(f.Size), total)]
	rows, err := s.store.listByIDs(ctx, window)
	if err != nil {
		return nil, 0, err
	}
	byID := make(map[int64]video, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]video, 0, len(window))
	for _, id := range window {
		if row, ok := byID[id]; ok && (f.Mine || row.Status == statusPublished) {
			ordered = append(ordered, row)
		}
	}
	return ordered, total, nil
}

func (s *Service) buildItems(ctx context.Context, rows []video, withParts bool) ([]videoItem, error) {
	ids := make([]int64, 0, len(rows))
	var categoryIDs []int64
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.CategoryID != nil {
			categoryIDs = append(categoryIDs, int64(*row.CategoryID))
		}
	}
	authors, err := s.users.PublicSummariesByIDs(ctx, uniqueAuthorIDs(rows))
	if err != nil {
		return nil, err
	}
	categories, err := s.store.categoriesByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, err
	}
	stats, err := s.store.statsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	categoryMap := make(map[int32]Category, len(categories))
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}
	statMap := make(map[int64]statistic, len(stats))
	for _, stat := range stats {
		statMap[stat.VideoID] = stat
	}
	partMap := make(map[int64][]part)
	var tags map[int64][]string
	if withParts {
		parts, err := s.store.partsByVideoIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, p := range parts {
			partMap[p.VideoID] = append(partMap[p.VideoID], p)
		}
		tags, err = s.store.tagsByVideoIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
	}
	items := make([]videoItem, 0, len(rows))
	for _, row := range rows {
		item := videoItem{ID: row.ID, BVID: row.BVID, Title: row.Title, Description: row.Description, CoverURL: row.CoverURL,
			DurationSec: row.DurationSec, Status: row.Status, PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt,
			Stat: statMap[row.ID]}
		if withParts {
			parts := partMap[row.ID]
			if parts == nil {
				parts = []part{}
			}
			names := tags[row.ID]
			item.Parts, item.Tags = &parts, &names
		}
		if author, ok := authors[row.AuthorID]; ok {
			item.Author = &author
		}
		if row.CategoryID != nil {
			if cat, ok := categoryMap[*row.CategoryID]; ok {
				item.Category = &cat
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) DetailByCode(ctx context.Context, code string, viewer int64) (videoDetail, error) {
	// 共享缓存只收录可公开访问的详情。
	result := s.detailLoads.DoChan(code, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.publicDetail(loadCtx, code)
	})
	select {
	case <-ctx.Done():
		return videoDetail{}, ctx.Err()
	case loaded := <-result:
		if loaded.Err != nil {
			return videoDetail{}, loaded.Err
		}
		if detail := loaded.Val.(*videoDetail); detail != nil {
			return *detail, nil
		}
	}
	if viewer != 0 {
		row, err := s.store.byCode(ctx, code, false)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return videoDetail{}, err
		}
		if err == nil && row.AuthorID == viewer && row.Status != statusWithdrawn {
			return s.buildDetail(ctx, row)
		}
	}
	return videoDetail{}, errcode.VideoNotFound
}

func (s *Service) publicDetail(ctx context.Context, code string) (*videoDetail, error) {
	data, version, err := s.store.cachedDetail(ctx, code)
	if err != nil {
		return nil, err
	}
	if data != "" {
		var detail *videoDetail
		if err := json.Unmarshal([]byte(data), &detail); err != nil {
			return nil, fmt.Errorf("读取视频详情缓存: %w", err)
		}
		return detail, nil
	}
	row, err := s.store.byCode(ctx, code, false)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var detail *videoDetail
	ttl := 15 * time.Second
	if err == nil && row.Status == statusPublished {
		value, err := s.buildDetail(ctx, row)
		if err != nil {
			return nil, err
		}
		detail = &value
		ttl = 3*time.Minute + time.Duration(rand.IntN(60))*time.Second
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	if err := s.store.cacheDetail(ctx, code, version, string(payload), ttl); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *Service) buildDetail(ctx context.Context, row video) (videoDetail, error) {
	// 详情沿用列表的作者与统计组装，并额外读取标签和选集。
	items, err := s.buildItems(ctx, []video{row}, false)
	if err != nil {
		return videoDetail{}, err
	}
	parts, err := s.store.partsByVideoIDs(ctx, []int64{row.ID})
	if err != nil {
		return videoDetail{}, err
	}
	tags, err := s.store.tagsByVideoIDs(ctx, []int64{row.ID})
	if err != nil {
		return videoDetail{}, err
	}
	return videoDetail{videoItem: items[0], Parts: parts, Tags: tags[row.ID]}, nil
}

var setDetailIfCurrent = redis.NewScript(`
local generation = redis.call('GET', KEYS[2]) or ''
if generation == ARGV[1] then
    redis.call('SET', KEYS[1], ARGV[2], 'PX', ARGV[3])
end
return 0
`)

func detailCacheKeys(code string) (string, string) {
	prefix := "video:detail:{" + code + "}"
	return prefix, prefix + ":generation"
}

func (s *Store) cachedDetail(ctx context.Context, code string) (string, string, error) {
	key, generation := detailCacheKeys(code)
	values, err := s.redis.MGet(ctx, key, generation).Result()
	if err != nil {
		return "", "", fmt.Errorf("读取视频详情缓存: %w", err)
	}
	data, _ := values[0].(string)
	version, _ := values[1].(string)
	return data, version, nil
}

func (s *Store) cacheDetail(ctx context.Context, code, version, data string, ttl time.Duration) error {
	key, generation := detailCacheKeys(code)
	return setDetailIfCurrent.Run(ctx, s.redis, []string{key, generation}, version, data, ttl.Milliseconds()).Err()
}

func (s *Store) invalidateDetail(ctx context.Context, code, version string) error {
	key, generation := detailCacheKeys(code)
	// 更新代号并删除缓存，阻止编辑前开始的读取写回旧内容。
	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, generation, version, 10*time.Minute)
		pipe.Del(ctx, key)
		return nil
	})
	return err
}
