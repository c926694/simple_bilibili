package user

import (
	"time"

	"simple_bilibili/backend/internal/pkg/jwt"
)

type registerInput struct {
	Username string  `json:"username"`
	Password string  `json:"password"`
	Nickname string  `json:"nickname"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
}

type loginInput struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type refreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type profileInput struct {
	Nickname  string  `json:"nickname"`
	Bio       string  `json:"bio"`
	AvatarURL string  `json:"avatar_url"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
}

type passwordInput struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type profileResponse struct {
	ID             int64     `db:"id" json:"id,string"`
	Username       string    `db:"username" json:"username"`
	Nickname       string    `db:"nickname" json:"nickname"`
	Email          *string   `db:"email" json:"email"`
	Phone          *string   `db:"phone" json:"phone"`
	AvatarURL      string    `db:"avatar_url" json:"avatar_url"`
	Bio            string    `db:"bio" json:"bio"`
	CoinBalance    int       `db:"coin_balance" json:"coin_balance"`
	FollowerCount  int       `db:"follower_count" json:"follower_count"`
	FollowingCount int       `db:"following_count" json:"following_count"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type publicProfileResponse struct {
	ID             int64     `db:"id" json:"id,string"`
	Nickname       string    `db:"nickname" json:"nickname"`
	AvatarURL      string    `db:"avatar_url" json:"avatar_url"`
	Bio            string    `db:"bio" json:"bio"`
	FollowerCount  int       `db:"follower_count" json:"follower_count"`
	FollowingCount int       `db:"following_count" json:"following_count"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	AccessExpiresIn  int64  `json:"access_expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

func tokenData(pair jwt.Pair) tokenResponse {
	return tokenResponse{
		AccessToken: pair.Access.Value, RefreshToken: pair.Refresh.Value,
		AccessExpiresIn: pair.Access.ExpiresIn, RefreshExpiresIn: pair.Refresh.ExpiresIn,
	}
}

type authResponse struct {
	tokenResponse
	User profileResponse `json:"user"`
}
