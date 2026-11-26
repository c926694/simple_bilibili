package user

type profile struct {
	ID       int64  `db:"id"`
	Nickname string `db:"nickname"`
}
