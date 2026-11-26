package user

import "time"

const (
	statusActive = 1
	initialCoins = 10
)

type account struct {
	profileResponse
	PasswordHash string `db:"password_hash"`
	Status       uint8  `db:"status"`
}

type accessSession struct {
	UserID    int64
	JTI       string
	ExpiresAt time.Time
}

type profile struct {
	ID       int64  `db:"id"`
	Nickname string `db:"nickname"`
}
