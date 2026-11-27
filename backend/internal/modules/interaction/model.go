package interaction

import "time"

type interactionRow struct {
	ID        int64 `db:"id"`
	Liked     bool  `db:"liked"`
	CoinCount int   `db:"coin_count"`
}

type ledgerRow struct {
	VideoID      int64 `db:"video_id"`
	CoinCount    int   `db:"coin_count"`
	BalanceAfter int   `db:"balance_after"`
}

type Folder struct {
	ID        int64     `db:"id" json:"id,string"`
	UserID    int64     `db:"user_id" json:"user_id,string"`
	Title     string    `db:"title" json:"title"`
	IsDefault bool      `db:"is_default" json:"is_default"`
	IsPublic  bool      `db:"is_public" json:"is_public"`
	ItemCount int       `db:"item_count" json:"item_count"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
