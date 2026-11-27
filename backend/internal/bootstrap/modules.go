package bootstrap

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/comment"
	"simple_bilibili/backend/internal/modules/danmaku"
	"simple_bilibili/backend/internal/modules/interaction"
	"simple_bilibili/backend/internal/modules/recommend"
	"simple_bilibili/backend/internal/modules/social"
	"simple_bilibili/backend/internal/modules/upload"
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/pkg/jwt"
	"simple_bilibili/backend/internal/pkg/snowflake"
	"simple_bilibili/backend/internal/ws"
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

func NewModules(db *sqlx.DB, client *redis.Client, objects *minio.Client, publisher *rocketmq.Producer, tokens *jwt.Manager, ids *snowflake.Generator, logger *slog.Logger, hub *ws.Hub) Modules {
	users := user.NewService(user.NewStore(db, client), objects, tokens, ids)
	uploads := upload.NewService(upload.NewStore(db), objects, ids)
	videos := video.NewService(video.NewStore(db, client), users, uploads, objects, publisher, ids, logger)

	return Modules{
		User:        users,
		Video:       videos,
		Upload:      uploads,
		Danmaku:     danmaku.NewService(videos, danmaku.NewStore(db, client), hub, publisher, ids, logger),
		Comment:     comment.NewService(users, videos),
		Interaction: interaction.NewService(users, videos),
		Social:      social.NewService(users, videos),
		Recommend:   recommend.NewService(users, videos),
	}
}
