package bootstrap

import (
	"github.com/jmoiron/sqlx"

	"simple_bilibili/backend/internal/modules/comment"
	"simple_bilibili/backend/internal/modules/danmaku"
	"simple_bilibili/backend/internal/modules/interaction"
	"simple_bilibili/backend/internal/modules/recommend"
	"simple_bilibili/backend/internal/modules/social"
	"simple_bilibili/backend/internal/modules/upload"
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/modules/video"
)

type Modules struct {
	User        *user.Service
	Video       *video.Service
	Upload      *upload.Service
	Danmaku     *danmaku.Service
	Comment     *comment.Service
	Interaction *interaction.Service
	Social      *social.Service
	Recommend   *recommend.Service
}

func NewModules(db *sqlx.DB) Modules {
	users := user.NewService(user.NewStore(db))
	videos := video.NewService(video.NewStore(db), users)

	return Modules{
		User:        users,
		Video:       videos,
		Upload:      upload.NewService(videos),
		Danmaku:     danmaku.NewService(videos),
		Comment:     comment.NewService(users, videos),
		Interaction: interaction.NewService(users, videos),
		Social:      social.NewService(users, videos),
		Recommend:   recommend.NewService(users, videos),
	}
}
