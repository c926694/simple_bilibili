package interaction

import (
	"context"
	"fmt"
	"math/bits"
	"time"

	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/tx"
)

const bitmapTTL = 93 * 24 * time.Hour

type checkinResponse struct {
	CheckedInToday bool   `json:"checked_in_today"`
	JustCheckedIn  bool   `json:"just_checked_in"`
	Month          string `json:"month"`
	MonthDays      int    `json:"month_days"`
	StreakDays     int    `json:"streak_days"`
	CheckedDates   []int  `json:"checked_dates"`
	CoinBalance    int    `json:"coin_balance"`
	RewardCoins    int    `json:"reward_coins,omitempty"`
}

func shanghaiNow() (time.Time, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().In(location), nil
}

func bitmapKey(userID int64, month time.Time) string {
	return fmt.Sprintf("checkin:{%d}:%s", userID, month.Format("200601"))
}

func monthStart(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
}

func (s *Service) CheckinStatus(ctx context.Context, userID int64) (checkinResponse, error) {
	day, err := shanghaiNow()
	if err != nil {
		return checkinResponse{}, err
	}
	balance, err := s.users.CoinBalance(ctx, userID)
	if err != nil {
		return checkinResponse{}, err
	}
	return s.checkinStatusAt(ctx, userID, day, balance)
}

func (s *Service) EnsureDailyCheckin(ctx context.Context, userID int64) (justCheckedIn bool, balance int, err error) {
	day, err := shanghaiNow()
	if err != nil {
		return false, 0, err
	}
	month := monthStart(day)
	key := bitmapKey(userID, month)
	offset := int64(day.Day() - 1)
	bit, err := s.store.redis.GetBit(ctx, key, offset).Result()
	if err != nil || bit == 1 {
		// 已签到直接使用位图，余额仅在本次发放奖励时返回。
		return false, 0, err
	}

	// 缺键时先恢复当月记录，再判定是否需要发放奖励。
	if _, err := s.ensureBitmap(ctx, userID, month); err != nil {
		return false, 0, err
	}
	bit, err = s.store.redis.GetBit(ctx, key, offset).Result()
	if err != nil || bit == 1 {
		return false, 0, err
	}

	err = s.userTx(ctx, userID, func(ctx context.Context) error {
		// 唯一日期记录负责去重，签到与奖励在同一事务提交。
		res, err := tx.Executor(ctx, s.store.db).ExecContext(ctx,
			"INSERT IGNORE INTO user_checkin (id,user_id,checkin_date,reward_coins) VALUES (?,?,?,1)",
			s.ids.Next(), userID, day.Format("2006-01-02"))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			return err
		}
		balance, err = s.users.ChangeCoins(ctx, userID, 1)
		justCheckedIn = err == nil
		return err
	})
	if err != nil {
		return false, 0, err
	}

	// 重复请求也修复位图，确保数据库提交后异常退出可以恢复。
	if _, err := s.ensureBitmap(ctx, userID, month); err != nil {
		return false, 0, err
	}
	_, err = s.store.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.SetBit(ctx, key, offset, 1)
		pipe.Expire(ctx, key, bitmapTTL)
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	return justCheckedIn, balance, nil
}

func (s *Service) checkinStatusAt(ctx context.Context, userID int64, day time.Time, balance int) (checkinResponse, error) {
	month := monthStart(day)
	key, err := s.ensureBitmap(ctx, userID, month)
	if err != nil {
		return checkinResponse{}, err
	}
	var todayCmd, totalCmd *redis.IntCmd
	var maskCmd *redis.IntSliceCmd
	if _, err := s.store.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		todayCmd = pipe.GetBit(ctx, key, int64(day.Day()-1))
		totalCmd = pipe.BitCount(ctx, key, nil)
		maskCmd = pipe.BitField(ctx, key, "GET", fmt.Sprintf("u%d", day.Day()), "0")
		return nil
	}); err != nil {
		return checkinResponse{}, err
	}
	today, total, mask := todayCmd.Val(), totalCmd.Val(), uint64(maskCmd.Val()[0])
	dates := make([]int, 0, int(total))
	for date := 1; date <= day.Day(); date++ {
		if mask&(1<<uint(day.Day()-date)) != 0 {
			dates = append(dates, date)
		}
	}
	start := day.Day()
	if today == 0 {
		start--
		mask >>= 1
	}
	// 当前月前缀连续签到时接续计算上一个月。
	streak, err := s.checkinStreak(ctx, userID, month, mask, start)
	if err != nil {
		return checkinResponse{}, err
	}
	return checkinResponse{CheckedInToday: today == 1, Month: month.Format("2006-01"),
		MonthDays: month.AddDate(0, 1, -1).Day(), StreakDays: streak, CheckedDates: dates, CoinBalance: balance}, nil
}

func (s *Service) bitmapMask(ctx context.Context, key string, day int) (uint64, error) {
	values, err := s.store.redis.BitField(ctx, key, "GET", fmt.Sprintf("u%d", day), "0").Result()
	if err != nil {
		return 0, err
	}
	return uint64(values[0]), nil
}

func (s *Service) checkinStreak(ctx context.Context, userID int64, month time.Time, mask uint64, days int) (int, error) {
	streak := 0
	for {
		run := min(days, bits.TrailingZeros64(^mask))
		streak += run
		if run < days {
			return streak, nil
		}
		previous := month.AddDate(0, 0, -1)
		month = monthStart(previous)
		key, err := s.ensureBitmap(ctx, userID, month)
		if err != nil {
			return 0, err
		}
		days = previous.Day()
		mask, err = s.bitmapMask(ctx, key, days)
		if err != nil {
			return 0, err
		}
	}
}

func (s *Service) ensureBitmap(ctx context.Context, userID int64, month time.Time) (string, error) {
	key := bitmapKey(userID, month)
	count, err := s.store.redis.Exists(ctx, key).Result()
	if err != nil {
		return "", err
	}
	if count > 0 {
		return key, nil
	}
	var days []int
	err = s.store.db.SelectContext(ctx, &days, `SELECT DAY(checkin_date) FROM user_checkin
		WHERE user_id = ? AND checkin_date >= ? AND checkin_date < ?`,
		userID, month.Format("2006-01-02"), month.AddDate(0, 1, 0).Format("2006-01-02"))
	if err != nil {
		return "", err
	}
	payload := make([]byte, 4)
	for _, day := range days {
		offset := day - 1
		payload[offset/8] |= 1 << uint(7-offset%8)
	}
	return key, s.store.redis.SetNX(ctx, key, payload, bitmapTTL).Err()
}
