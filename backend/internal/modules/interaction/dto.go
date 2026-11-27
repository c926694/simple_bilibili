package interaction

import (
	"strconv"
	"time"

	"simple_bilibili/backend/internal/modules/video"
)

type stateResponse struct {
	Liked     bool           `json:"liked"`
	CoinCount int            `json:"coin_count"`
	Favorited bool           `json:"favorited"`
	FolderIDs []string       `json:"favorite_folder_ids"`
	Balance   int            `json:"coin_balance"`
	Progress  *watchProgress `json:"watch_progress"`
}

type watchProgress struct {
	PartID    int64     `json:"part_id,string"`
	PartIndex int       `json:"part_index"`
	Progress  int       `json:"progress_sec"`
	WatchedAt time.Time `json:"watched_at"`
}

type likeResponse struct {
	Liked bool `json:"liked"`
	Delta int  `json:"delta"`
}

type coinResponse struct {
	CoinCount  int `json:"coin_count"`
	AddedCoins int `json:"added_coins"`
	Balance    int `json:"coin_balance"`
}

type tripleResponse struct {
	coinResponse
	Liked             bool     `json:"liked"`
	LikeDelta         int      `json:"like_delta"`
	CoinSkippedReason string   `json:"coin_skipped_reason"`
	Favorited         bool     `json:"favorited"`
	FavoriteDelta     int      `json:"favorite_delta"`
	FolderIDs         []string `json:"favorite_folder_ids"`
}

type favoriteResponse struct {
	Favorited bool     `json:"favorited"`
	FolderIDs []string `json:"favorite_folder_ids"`
	Delta     int      `json:"delta"`
}

type countedResponse struct {
	Counted bool `json:"counted"`
}

type folderInput struct {
	Title    string `json:"title"`
	IsPublic bool   `json:"is_public"`
}

type page struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func (p page) bounds(total int) (int, int) {
	start := min(total, (p.Page-1)*p.PageSize)
	return start, min(total, start+p.PageSize)
}

type folderVideos struct {
	Folder Folder          `json:"folder"`
	Items  []video.Summary `json:"items"`
	Total  int             `json:"total"`
	page
}

func stringIDs(ids []int64) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, strconv.FormatInt(id, 10))
	}
	return result
}
