package interaction

import (
	"context"
	"errors"
	"log/slog"

	"simple_bilibili/backend/internal/infra/rocketmq"
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/mq"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/snowflake"
	"simple_bilibili/backend/internal/pkg/tx"
)

type Service struct {
	users     *user.Service
	videos    *video.Service
	store     *Store
	publisher *rocketmq.Producer
	ids       *snowflake.Generator
	logger    *slog.Logger
	wake      chan struct{}
}

func NewService(users *user.Service, videos *video.Service, store *Store, publisher *rocketmq.Producer, ids *snowflake.Generator, logger *slog.Logger) *Service {
	return &Service{users: users, videos: videos, store: store, publisher: publisher, ids: ids, logger: logger, wake: make(chan struct{}, 1)}
}

func (s *Service) userTx(ctx context.Context, userID int64, run func(context.Context) error) error {
	err := tx.WithTx(ctx, s.store.db, func(ctx context.Context) error {
		// 同一用户的互动更新使用固定锁顺序，避免收藏夹删除与添加互相覆盖。
		if err := s.users.LockForUpdate(ctx, userID); err != nil {
			return err
		}
		return run(ctx)
	})
	if err == nil {
		s.wakeOutbox()
	}
	return err
}

func (s *Service) wakeOutbox() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) delta(ctx context.Context, videoID int64, like, coin, favorite, share int) error {
	if like == 0 && coin == 0 && favorite == 0 && share == 0 {
		return nil
	}
	return s.store.saveDelta(ctx, mq.VideoStatDelta{EventID: s.ids.Next(), VideoID: videoID,
		LikeDelta: int64(like), CoinDelta: int64(coin), FavoriteDelta: int64(favorite), ShareDelta: int64(share)})
}

func (s *Service) State(ctx context.Context, userID int64, bvid string) (stateResponse, error) {
	video, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return stateResponse{}, err
	}
	row, err := s.store.interaction(ctx, userID, video.ID)
	if err != nil {
		return stateResponse{}, err
	}
	folders, err := s.store.favoriteIDs(ctx, userID, video.ID)
	if err != nil {
		return stateResponse{}, err
	}
	balance, err := s.users.CoinBalance(ctx, userID)
	if err != nil {
		return stateResponse{}, err
	}
	// 进度与历史接口采用相同的热数据及删除标记。
	progress, err := s.progressForVideo(ctx, userID, video.ID)
	if err != nil {
		return stateResponse{}, err
	}
	return stateResponse{Liked: row.Liked, CoinCount: row.CoinCount, Favorited: len(folders) > 0, FolderIDs: stringIDs(folders), Balance: balance, Progress: progress}, nil
}

func (s *Service) Like(ctx context.Context, userID int64, bvid string, wanted bool) (likeResponse, error) {
	video, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return likeResponse{}, err
	}
	result := likeResponse{Liked: wanted}
	err = s.userTx(ctx, userID, func(ctx context.Context) error {
		row, err := s.store.interaction(ctx, userID, video.ID)
		if err != nil {
			return err
		}
		if row.Liked == wanted {
			return nil
		}
		if row.ID == 0 {
			row.ID = s.ids.Next()
		}
		row.Liked = wanted
		result.Delta = -1
		if wanted {
			result.Delta = 1
		}
		if err := s.store.saveInteraction(ctx, row, userID, video.ID); err != nil {
			return err
		}
		return s.delta(ctx, video.ID, result.Delta, 0, 0, 0)
	})
	return result, err
}

func (s *Service) Coin(ctx context.Context, userID int64, bvid string, count int, request string) (coinResponse, error) {
	video, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return coinResponse{}, err
	}
	if video.AuthorID == userID {
		return coinResponse{}, errcode.CannotCoinOwnVideo
	}
	var result coinResponse
	err = s.userTx(ctx, userID, func(ctx context.Context) error {
		row, err := s.store.interaction(ctx, userID, video.ID)
		if err != nil {
			return err
		}
		ledger, exists, err := s.store.ledger(ctx, userID, request)
		if err != nil {
			return err
		}
		if exists {
			if ledger.VideoID != video.ID || ledger.CoinCount != count {
				return errcode.InvalidParams
			}
			result = coinResponse{row.CoinCount, ledger.CoinCount, ledger.BalanceAfter}
			return nil
		}
		if row.CoinCount+count > 2 {
			return errcode.CoinLimitReached
		}
		// 同一事务完成扣币、流水和统计待投递记录。
		result.Balance, err = s.debit(ctx, userID, video.ID, count, request)
		if err != nil {
			return err
		}
		if row.ID == 0 {
			row.ID = s.ids.Next()
		}
		row.CoinCount += count
		result.CoinCount, result.AddedCoins = row.CoinCount, count
		if err := s.store.saveInteraction(ctx, row, userID, video.ID); err != nil {
			return err
		}
		return s.delta(ctx, video.ID, 0, count, 0, 0)
	})
	return result, err
}

func (s *Service) debit(ctx context.Context, userID, videoID int64, count int, request string) (int, error) {
	balance, err := s.users.ChangeCoins(ctx, userID, -count)
	if err != nil {
		return balance, err
	}
	if err := s.store.addLedger(ctx, s.ids.Next(), userID, videoID, request, count, balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *Service) Triple(ctx context.Context, userID int64, bvid, request string) (tripleResponse, error) {
	video, err := s.videos.PublishedVideo(ctx, bvid)
	if err != nil {
		return tripleResponse{}, err
	}
	result := tripleResponse{Liked: true, Favorited: true}
	err = s.userTx(ctx, userID, func(ctx context.Context) error {
		row, err := s.store.interaction(ctx, userID, video.ID)
		if err != nil {
			return err
		}
		if !row.Liked {
			row.Liked = true
			result.LikeDelta = 1
		}
		// 已收藏的视频保留其收藏夹，首次收藏才使用默认夹。
		folders, err := s.favoriteForTriple(ctx, userID, video.ID)
		if err != nil {
			return err
		}
		result.FolderIDs, result.FavoriteDelta = folders.FolderIDs, folders.Delta
		// 自己的视频和余额不足只跳过投币，赞与藏仍提交。
		if err := s.coinForTriple(ctx, userID, video, request, &row, &result); err != nil {
			return err
		}
		if row.ID == 0 {
			row.ID = s.ids.Next()
		}
		if err := s.store.saveInteraction(ctx, row, userID, video.ID); err != nil {
			return err
		}
		return s.delta(ctx, video.ID, result.LikeDelta, result.AddedCoins, result.FavoriteDelta, 0)
	})
	return result, err
}

func (s *Service) coinForTriple(ctx context.Context, userID int64, published video.PublishedVideo, request string, row *interactionRow, result *tripleResponse) error {
	ledger, exists, err := s.store.ledger(ctx, userID, request)
	if err != nil {
		return err
	}
	if exists && ledger.VideoID != published.ID {
		return errcode.InvalidParams
	}
	need := 2 - row.CoinCount
	result.CoinCount = row.CoinCount
	if need == 0 || exists {
		result.Balance, err = s.users.CoinBalance(ctx, userID)
		return err
	}
	if published.AuthorID == userID {
		result.CoinSkippedReason = "self_video"
		result.Balance, err = s.users.CoinBalance(ctx, userID)
		return err
	}
	result.Balance, err = s.debit(ctx, userID, published.ID, need, request)
	if errors.Is(err, errcode.InsufficientCoins) {
		result.CoinSkippedReason = "insufficient_coins"
		return nil
	}
	if err != nil {
		return err
	}
	row.CoinCount = 2
	result.CoinCount, result.AddedCoins = 2, need
	return nil
}
