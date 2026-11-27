package video

import (
	"time"

	"simple_bilibili/backend/internal/modules/user"
)

type Detail struct {
	ID             int64
	AuthorID       int64
	AuthorNickname string
	Title          string
}

type categoryListing struct {
	Category
	SortOrder int `db:"sort_order" json:"sort_order"`
}

type partInput struct {
	ID           int64  `json:"id,string"`
	PartID       int64  `json:"part_id,string"`
	UploadTaskID int64  `json:"upload_task_id,string"`
	Title        string `json:"title"`
}

type draftInput struct {
	UploadTaskID *int64       `json:"upload_task_id,string"`
	Title        *string      `json:"title"`
	Description  *string      `json:"description"`
	CategoryID   *int32       `json:"category_id"`
	CoverURL     *string      `json:"cover_url"`
	Tags         *[]string    `json:"tags"`
	Parts        *[]partInput `json:"parts"`
}

type listFilter struct {
	CategoryID int32
	AuthorID   int64
	Status     *uint8
	Mine       bool
	Keyword    string
	Sort       string
	Page       int
	Size       int
}

func (f listFilter) offset() int64 { return int64(f.Page-1) * int64(f.Size) }

type videoItem struct {
	ID          int64               `json:"id,string"`
	BVID        string              `json:"bv_id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	CoverURL    string              `json:"cover_url"`
	DurationSec int64               `json:"duration_sec"`
	Status      uint8               `json:"status"`
	PublishedAt *time.Time          `json:"published_at"`
	CreatedAt   time.Time           `json:"created_at"`
	Category    *Category           `json:"category"`
	Author      *user.PublicSummary `json:"author"`
	Stat        statistic           `json:"stat"`
	Parts       *[]part             `json:"parts,omitempty"`
	Tags        *[]string           `json:"tags,omitempty"`
}

type videoDetail struct {
	videoItem
	Tags  []string `json:"tags"`
	Parts []part   `json:"parts"`
}

type videoPage struct {
	Items    []videoItem `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

type userPage struct {
	Items []user.PublicProfile `json:"items"`
	Total int64                `json:"total"`
}

type searchResult struct {
	Videos videoPage `json:"videos"`
	Users  userPage  `json:"users"`
}
