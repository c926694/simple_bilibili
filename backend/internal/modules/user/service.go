package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"simple_bilibili/backend/internal/infra/minio"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/jwt"
	"simple_bilibili/backend/internal/pkg/password"
	"simple_bilibili/backend/internal/pkg/snowflake"
)

type Service struct {
	store   *Store
	objects *minio.Client
	tokens  *jwt.Manager
	ids     *snowflake.Generator
}

func NewService(store *Store, objects *minio.Client, tokens *jwt.Manager, ids *snowflake.Generator) *Service {
	return &Service{store: store, objects: objects, tokens: tokens, ids: ids}
}

func (s *Service) NicknamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := s.store.profilesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	nicknames := make(map[int64]string, len(rows))
	for _, row := range rows {
		nicknames[row.ID] = row.Nickname
	}
	return nicknames, nil
}

func (s *Service) Register(ctx context.Context, input registerInput) (authResponse, error) {
	if err := s.store.checkRegistration(ctx, input); err != nil {
		return authResponse{}, err
	}
	newAccount := account{profileResponse: profileResponse{
		ID: s.ids.Next(), Username: input.Username, Nickname: input.Nickname,
		Email: input.Email, Phone: input.Phone, AvatarURL: defaultAvatarURL, CoinBalance: initialCoins,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}, Status: statusActive}
	hash, err := password.Hash(input.Password)
	if err != nil {
		return authResponse{}, fmt.Errorf("生成注册密码哈希: %w", err)
	}
	newAccount.PasswordHash = hash

	var result authResponse
	err = s.store.inTransaction(ctx, func(ctx context.Context) error {
		if err := s.store.insert(ctx, newAccount); err != nil {
			return err
		}
		// 刷新会话写入失败时回退用户创建。
		var err error
		result, err = s.startSession(ctx, newAccount.profileResponse)
		return err
	})
	if err != nil {
		return authResponse{}, err
	}
	return result, nil
}

func (s *Service) Login(ctx context.Context, input loginInput) (authResponse, error) {
	var result authResponse
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 持有共享行锁直到会话写入完成，密码更新随后可以撤销此次登录的会话。
		accounts, err := s.store.loginAccounts(ctx, input.Account)
		if err != nil {
			return err
		}
		// 同一数字串可以同时是用户名和手机号，以匹配的密码确认账号。
		account, err := matchLogin(accounts, input.Password)
		if err != nil {
			return err
		}
		// 验证通过后签发凭证并记录本次刷新会话。
		result, err = s.startSession(ctx, account.profileResponse)
		return err
	})
	if err != nil {
		return authResponse{}, err
	}
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, value string) (tokenResponse, error) {
	claims, err := s.tokens.Parse(value, jwt.Refresh)
	if err != nil {
		return tokenResponse{}, errcode.InvalidRefresh
	}
	pair, err := s.tokens.Issue(claims.UserID)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("签发轮换凭证: %w", err)
	}
	rotated, err := s.store.saveSession(ctx, claims.UserID, claims.ID, pair.Refresh)
	if err != nil {
		return tokenResponse{}, err
	}
	if !rotated {
		return tokenResponse{}, errcode.InvalidRefresh
	}
	return tokenData(pair), nil
}

func (s *Service) Logout(ctx context.Context, access accessSession, refresh string) error {
	refreshJTI := ""
	if refresh != "" {
		if claims, err := s.tokens.Parse(refresh, jwt.Refresh); err == nil && claims.UserID == access.UserID {
			refreshJTI = claims.ID
		}
	}
	return s.store.revokeSessions(ctx, access, refreshJTI, false)
}

func (s *Service) Me(ctx context.Context, userID int64) (profileResponse, error) {
	result, err := s.store.profileByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return profileResponse{}, errcode.UserNotFound
	}
	return result, err
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, input profileInput) (profileResponse, error) {
	var result profileResponse
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		if err := s.store.updateProfile(ctx, userID, input); err != nil {
			return err
		}
		var err error
		result, err = s.Me(ctx, userID)
		return err
	})
	if err != nil {
		return profileResponse{}, err
	}
	return result, nil
}

func (s *Service) UploadAvatar(ctx context.Context, userID int64, input avatarInput) (profileResponse, error) {
	objectKey := fmt.Sprintf("avatars/%d/%d.%s", userID, s.ids.Next(), input.extension)
	if err := s.objects.PutObject(ctx, objectKey, input.file, input.size, input.contentType); err != nil {
		return profileResponse{}, err
	}

	var result profileResponse
	readyToCommit := false
	err := s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 仅更新头像地址，在同一事务读取完整资料。
		if err := s.store.updateAvatar(ctx, userID, "/storage/"+objectKey); err != nil {
			return err
		}
		var err error
		result, err = s.Me(ctx, userID)
		readyToCommit = err == nil
		return err
	})
	if err != nil {
		// 提交结果不明时保留对象，避免删除已写入资料的头像。
		if !readyToCommit {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			err = errors.Join(err, s.objects.RemoveObject(cleanupCtx, objectKey))
		}
		return profileResponse{}, err
	}
	return result, nil
}

func (s *Service) ChangePassword(ctx context.Context, access accessSession, input passwordInput) error {
	return s.store.inTransaction(ctx, func(ctx context.Context) error {
		// 排他行锁保证密码比对使用本次更新前的记录。
		oldHash, err := s.store.passwordForUpdate(ctx, access.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			return errcode.UserNotFound
		}
		if err != nil {
			return err
		}
		// 保留损坏哈希的内部错误，错误的原密码返回业务错误码。
		if err := checkPassword(oldHash, input.OldPassword, errcode.WrongPassword); err != nil {
			return err
		}
		hash, err := password.Hash(input.NewPassword)
		if err != nil {
			return fmt.Errorf("生成新密码哈希: %w", err)
		}
		if err := s.store.updatePassword(ctx, access.UserID, hash); err != nil {
			return err
		}
		// 撤销失败时回退密码更新，登录在行锁释放后读取新密码。
		return s.store.revokeSessions(ctx, access, "", true)
	})
}

func (s *Service) PublicProfile(ctx context.Context, userID int64) (publicProfileResponse, error) {
	result, err := s.store.publicProfileByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return publicProfileResponse{}, errcode.UserNotFound
	}
	return result, err
}

func (s *Service) AccessRevoked(ctx context.Context, jti string) (bool, error) {
	return s.store.accessRevoked(ctx, jti)
}

func (s *Service) startSession(ctx context.Context, p profileResponse) (authResponse, error) {
	pair, err := s.tokens.Issue(p.ID)
	if err != nil {
		return authResponse{}, fmt.Errorf("签发登录凭证: %w", err)
	}
	if _, err := s.store.saveSession(ctx, p.ID, "", pair.Refresh); err != nil {
		return authResponse{}, err
	}
	return authResponse{tokenResponse: tokenData(pair), User: p}, nil
}

func matchLogin(accounts []account, value string) (account, error) {
	failure := errcode.InvalidLogin
	if len(accounts) > 0 && accounts[0].Status != statusActive {
		failure = errcode.AccountBanned
	}
	for _, candidate := range accounts {
		if candidate.Status != statusActive {
			continue
		}
		err := checkPassword(candidate.PasswordHash, value, errcode.InvalidLogin)
		if err == nil {
			return candidate, nil
		}
		if !errors.Is(err, errcode.InvalidLogin) {
			return account{}, err
		}
	}
	return account{}, failure
}

func checkPassword(hash, value string, mismatch errcode.Code) error {
	err := password.Compare(hash, value)
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return mismatch
	}
	if err != nil {
		return fmt.Errorf("比对密码哈希: %w", err)
	}
	return nil
}
