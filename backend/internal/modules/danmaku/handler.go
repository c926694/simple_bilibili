package danmaku

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) send(c *gin.Context) {
	var input sendRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	validated, ok := validateSend(input)
	if !ok {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	data, err := h.service.Send(c.Request.Context(), c.GetInt64(middleware.UserIDKey), c.Param("bvid"), validated)
	h.respond(c, data, err)
}

func validateSend(input sendRequest) (sendInput, bool) {
	content := strings.TrimSpace(input.Content)
	if input.PartID <= 0 || content == "" || utf8.RuneCountInString(content) > 100 ||
		input.TimeOffsetMS == nil || *input.TimeOffsetMS < 0 || *input.TimeOffsetMS > math.MaxInt32 {
		return sendInput{}, false
	}
	result := sendInput{PartID: input.PartID, Content: content, TimeOffsetMS: *input.TimeOffsetMS, Mode: 1, FontSize: 25, Color: "#FFFFFF"}
	if input.Mode != nil {
		if *input.Mode < 1 || *input.Mode > 3 {
			return sendInput{}, false
		}
		result.Mode = *input.Mode
	}
	if input.FontSize != nil {
		if !allowedFontSize(*input.FontSize) {
			return sendInput{}, false
		}
		result.FontSize = *input.FontSize
	}
	if input.Color != nil {
		if !colorPattern.MatchString(*input.Color) {
			return sendInput{}, false
		}
		result.Color = strings.ToUpper(*input.Color)
	}
	return result, true
}

func allowedFontSize(n uint8) bool { return n == 18 || n == 25 || n == 36 }

func (h *Handler) history(c *gin.Context) {
	partID, err := strconv.ParseInt(c.Query("part_id"), 10, 64)
	if err != nil || partID <= 0 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	segment, err := strconv.ParseInt(c.DefaultQuery("segment", "0"), 10, 64)
	if err != nil || segment < 0 || segment > math.MaxInt64/segmentDurationMS-1 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	data, err := h.service.History(c.Request.Context(), c.Param("bvid"), partID, segment)
	h.respond(c, data, err)
}

func (h *Handler) connect(c *gin.Context) {
	id, err := h.service.videos.PublishedVideoID(c.Request.Context(), c.Param("bvid"))
	if errors.Is(err, errcode.VideoNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "校验视频房间", "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	h.service.hub.Serve(c.Writer, c.Request, id, c.GetInt64(middleware.UserIDKey))
}

func websocketCredentials(c *gin.Context) {
	if c.GetHeader("Authorization") == "" {
		// 浏览器通过子协议传递 Bearer，握手只确认 bilibili 子协议。
		for _, protocol := range websocket.Subprotocols(c.Request) {
			if token, ok := strings.CutPrefix(protocol, "bearer."); ok && token != "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
				break
			}
		}
	}
	c.Next()
}

func (h *Handler) respond(c *gin.Context, data any, err error) {
	if err == nil {
		response.OK(c, data)
		return
	}
	var code errcode.Code
	if errors.As(err, &code) {
		response.Fail(c, code, "")
		return
	}
	h.logger.ErrorContext(c.Request.Context(), "处理弹幕请求", "path", c.FullPath(), "error", err)
	response.Fail(c, errcode.InternalError, "")
}
