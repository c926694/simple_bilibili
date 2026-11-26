package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/tx"
)

type Store struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewStore(db *sqlx.DB, client *redis.Client) *Store {
	return &Store{db: db, redis: client}
}

func (s *Store) profilesByIDs(ctx context.Context, ids []int64) ([]profile, error) {
	rows := []profile{}
	if len(ids) == 0 {
		return rows, nil
	}

	query, args, err := sqlx.In("SELECT id, nickname FROM user WHERE id IN (?)", ids)
	if err != nil {
		return nil, fmt.Errorf("构造用户批量查询: %w", err)
	}
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &rows, s.db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("批量查询用户昵称: %w", err)
	}
	return rows, nil
}

const profileColumns = "id, username, nickname, email, phone, avatar_url, bio, coin_balance, follower_count, following_count, created_at"
const publicProfileColumns = "id, nickname, avatar_url, bio, follower_count, following_count, created_at"

func (s *Store) inTransaction(ctx context.Context, run func(context.Context) error) error {
	return tx.WithTx(ctx, s.db, run)
}

func (s *Store) checkRegistration(ctx context.Context, input registerInput) error {
	const query = `SELECT EXISTS(SELECT 1 FROM user WHERE username = ?),
		EXISTS(SELECT 1 FROM user WHERE email = ?),
		EXISTS(SELECT 1 FROM user WHERE phone = ?)`
	var usernameTaken, emailTaken, phoneTaken bool
	err := s.db.QueryRowxContext(ctx, query, input.Username, input.Email, input.Phone).Scan(&usernameTaken, &emailTaken, &phoneTaken)
	if err != nil {
		return fmt.Errorf("检查注册信息唯一性: %w", err)
	}
	switch {
	case usernameTaken:
		return errcode.UsernameExists
	case emailTaken:
		return errcode.EmailExists
	case phoneTaken:
		return errcode.PhoneExists
	default:
		return nil
	}
}

func (s *Store) insert(ctx context.Context, a account) error {
	const query = `INSERT INTO user
		(id, username, nickname, email, phone, avatar_url, password_hash, coin_balance, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, query, a.ID, a.Username, a.Nickname, a.Email, a.Phone,
		a.AvatarURL, a.PasswordHash, a.CoinBalance, a.Status, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("创建用户: %w", duplicateAccountError(err))
	}
	return nil
}

func (s *Store) loginAccounts(ctx context.Context, login string) ([]account, error) {
	const query = `SELECT ` + profileColumns + `, password_hash, status
		FROM user WHERE username = ? OR email = ? OR phone = ?
		ORDER BY CASE WHEN username = ? THEN 0 WHEN email = ? THEN 1 ELSE 2 END
		FOR SHARE`
	var accounts []account
	if err := sqlx.SelectContext(ctx, tx.Executor(ctx, s.db), &accounts, query, login, login, login, login, login); err != nil {
		return nil, fmt.Errorf("按登录账号读取用户: %w", err)
	}
	return accounts, nil
}

func (s *Store) profileByID(ctx context.Context, userID int64) (profileResponse, error) {
	var result profileResponse
	query := "SELECT " + profileColumns + " FROM user WHERE id = ?"
	if err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &result, query, userID); err != nil {
		return profileResponse{}, fmt.Errorf("读取个人资料: %w", err)
	}
	return result, nil
}

func (s *Store) publicProfileByID(ctx context.Context, userID int64) (publicProfileResponse, error) {
	var result publicProfileResponse
	query := "SELECT " + publicProfileColumns + " FROM user WHERE id = ?"
	if err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &result, query, userID); err != nil {
		return publicProfileResponse{}, fmt.Errorf("读取公开主页: %w", err)
	}
	return result, nil
}

func (s *Store) updateProfile(ctx context.Context, userID int64, input profileInput) error {
	const query = "UPDATE user SET nickname = ?, bio = ?, avatar_url = ?, email = ?, phone = ? WHERE id = ?"
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, query, input.Nickname, input.Bio, input.AvatarURL, input.Email, input.Phone, userID)
	if err != nil {
		return fmt.Errorf("更新个人资料: %w", duplicateAccountError(err))
	}
	return nil
}

func (s *Store) updateAvatar(ctx context.Context, userID int64, url string) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE user SET avatar_url = ? WHERE id = ?", url, userID)
	if err != nil {
		return fmt.Errorf("更新头像地址: %w", err)
	}
	return nil
}

func (s *Store) passwordForUpdate(ctx context.Context, userID int64) (string, error) {
	var hash string
	err := sqlx.GetContext(ctx, tx.Executor(ctx, s.db), &hash, "SELECT password_hash FROM user WHERE id = ? FOR UPDATE", userID)
	if err != nil {
		return "", fmt.Errorf("读取待修改的密码哈希: %w", err)
	}
	return hash, nil
}

func (s *Store) updatePassword(ctx context.Context, userID int64, hash string) error {
	_, err := tx.Executor(ctx, s.db).ExecContext(ctx, "UPDATE user SET password_hash = ? WHERE id = ?", hash, userID)
	if err != nil {
		return fmt.Errorf("更新密码哈希: %w", err)
	}
	return nil
}

func duplicateAccountError(err error) error {
	var constraint *mysql.MySQLError
	if !errors.As(err, &constraint) || constraint.Number != 1062 {
		return err
	}
	for _, conflict := range []struct {
		index string
		code  errcode.Code
	}{
		{"uk_user_username", errcode.UsernameExists},
		{"uk_user_email", errcode.EmailExists},
		{"uk_user_phone", errcode.PhoneExists},
	} {
		if strings.HasSuffix(constraint.Message, "."+conflict.index+"'") ||
			strings.HasSuffix(constraint.Message, " '"+conflict.index+"'") {
			return conflict.code
		}
	}
	return err
}
