package user

import (
	"errors"
	"io"
	"log/slog"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

const (
	maxUsernameLength = 64
	maxNicknameLength = 64
)

var phonePattern = regexp.MustCompile(`^[0-9]{7,15}$`)

type Handler struct {
	service *Service
	logger  *slog.Logger
	checkin middleware.DailyCheckinHook
}

func NewHandler(service *Service, logger *slog.Logger, checkin middleware.DailyCheckinHook) *Handler {
	return &Handler{service: service, logger: logger, checkin: checkin}
}

func (h *Handler) register(c *gin.Context) {
	var input registerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || utf8.RuneCountInString(input.Username) > maxUsernameLength ||
		input.Password == "" || utf8.RuneCountInString(input.Nickname) > maxNicknameLength ||
		!validContact(input.Email, input.Phone) {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	if input.Nickname == "" {
		input.Nickname = input.Username
	}
	input.Email, input.Phone = emptyAsNull(input.Email), emptyAsNull(input.Phone)
	result, err := h.service.Register(c.Request.Context(), input)
	h.respond(c, result, err)
}

func (h *Handler) login(c *gin.Context) {
	var input loginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	input.Account = strings.TrimSpace(input.Account)
	if input.Account == "" || input.Password == "" {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	result, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	ctx := c.Request.Context()
	justCheckedIn, balance, err := h.checkin(ctx, result.User.ID)
	if err == nil && !justCheckedIn {
		// 另一条请求可能在登录读取资料后已经发放硬币。
		balance, err = h.service.CoinBalance(ctx, result.User.ID)
	}
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	result.User.CoinBalance = balance
	result.JustCheckedIn = &justCheckedIn
	h.respond(c, result, nil)
}

func (h *Handler) refresh(c *gin.Context) {
	var input refreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), input.RefreshToken)
	h.respond(c, result, err)
}

func (h *Handler) logout(c *gin.Context) {
	var input refreshInput
	if err := c.ShouldBindJSON(&input); err != nil && !errors.Is(err, io.EOF) {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	err := h.service.Logout(c.Request.Context(), currentAccess(c), input.RefreshToken)
	h.respond(c, nil, err)
}

func (h *Handler) me(c *gin.Context) {
	result, err := h.service.Me(c.Request.Context(), c.GetInt64(middleware.UserIDKey))
	h.respond(c, result, err)
}

func (h *Handler) updateProfile(c *gin.Context) {
	var input profileInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Nickname == "" ||
		utf8.RuneCountInString(input.Nickname) > maxNicknameLength || utf8.RuneCountInString(input.Bio) > 200 ||
		!validAvatar(input.AvatarURL) || !validContact(input.Email, input.Phone) {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	if input.AvatarURL == "" {
		input.AvatarURL = defaultAvatarURL
	}
	input.Email, input.Phone = emptyAsNull(input.Email), emptyAsNull(input.Phone)
	result, err := h.service.UpdateProfile(c.Request.Context(), c.GetInt64(middleware.UserIDKey), input)
	h.respond(c, result, err)
}

func (h *Handler) changePassword(c *gin.Context) {
	var input passwordInput
	if err := c.ShouldBindJSON(&input); err != nil || input.OldPassword == "" ||
		input.NewPassword == "" || input.OldPassword == input.NewPassword {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	err := h.service.ChangePassword(c.Request.Context(), currentAccess(c), input)
	h.respond(c, nil, err)
}

func (h *Handler) publicProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	result, err := h.service.PublicProfile(c.Request.Context(), id)
	h.respond(c, result, err)
}

func (h *Handler) respond(c *gin.Context, result any, err error) {
	if err == nil {
		response.OK(c, result)
		return
	}
	var code errcode.Code
	if errors.As(err, &code) {
		response.Fail(c, code, "")
		return
	}
	h.logger.ErrorContext(c.Request.Context(), "用户请求执行失败", "path", c.FullPath(), "error", err)
	response.Fail(c, errcode.InternalError, "")
}

func currentAccess(c *gin.Context) accessSession {
	return accessSession{
		UserID: c.GetInt64(middleware.UserIDKey), JTI: c.GetString(middleware.JTIKey),
		ExpiresAt: c.GetTime(middleware.ExpiryKey),
	}
}

func validContact(email, phone *string) bool {
	if email != nil && *email != "" {
		address, err := mail.ParseAddress(*email)
		if err != nil || address.Address != *email || utf8.RuneCountInString(*email) > 128 {
			return false
		}
	}
	return phone == nil || *phone == "" || phonePattern.MatchString(*phone)
}

func validAvatar(value string) bool {
	return utf8.RuneCountInString(value) <= 512 && (value == "" ||
		strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/"))
}

func emptyAsNull(value *string) *string {
	if value != nil && *value == "" {
		return nil
	}
	return value
}
