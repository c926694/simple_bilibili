package upload

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/media"
	"simple_bilibili/backend/internal/pkg/response"
)

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var imagePrefixes = map[string]string{"avatar": "avatars", "cover": "covers", "dynamic": "dynamics"}
var videoMediaTypes = map[string]string{
	".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime", ".mkv": "video/x-matroska",
}

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) imagePresign(c *gin.Context) {
	var input imageInput
	if err := c.ShouldBindJSON(&input); err != nil || input.FileSize <= 0 || input.FileSize > media.MaxImageSize {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	prefix := imagePrefixes[input.BizType]
	extension := media.ImageExtension(input.ContentType)
	if prefix == "" || extension == "" {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	data, err := h.service.ImagePresign(c.Request.Context(), c.GetInt64(middleware.UserIDKey), prefix, extension, input.ContentType)
	h.respond(c, data, err)
}

func (h *Handler) initVideo(c *gin.Context) {
	var input videoInput
	if err := c.ShouldBindJSON(&input); err != nil || !hashPattern.MatchString(input.FileHash) ||
		input.FileSize <= 0 || input.FileSize > maxVideoSize || utf8.RuneCountInString(input.FileName) > 255 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	input.extension = strings.ToLower(path.Ext(input.FileName))
	contentType := videoMediaTypes[input.extension]
	if contentType == "" {
		response.Fail(c, errcode.UnsupportedVideoFormat, "")
		return
	}
	if input.ContentType == "" {
		input.ContentType = contentType
	} else {
		value, _, err := mime.ParseMediaType(input.ContentType)
		if err != nil {
			response.Fail(c, errcode.InvalidParams, "")
			return
		}
		input.ContentType = value
	}
	data, err := h.service.InitVideo(c.Request.Context(), c.GetInt64(middleware.UserIDKey), input)
	h.respond(c, data, err)
}

func (h *Handler) partURLs(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var input partURLInput
	if err := c.ShouldBindJSON(&input); err != nil || len(input.PartNumbers) == 0 || len(input.PartNumbers) > 20 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	seen := make(map[int]bool, len(input.PartNumbers))
	for _, number := range input.PartNumbers {
		if number < 1 || number > maxVideoParts || seen[number] {
			response.Fail(c, errcode.InvalidParams, "")
			return
		}
		seen[number] = true
	}
	data, err := h.service.PartURLs(c.Request.Context(), c.GetInt64(middleware.UserIDKey), id, input.PartNumbers)
	h.respond(c, data, err)
}

func (h *Handler) complete(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	var input completeInput
	if err := c.ShouldBindJSON(&input); (err != nil && !errors.Is(err, io.EOF)) || len(input.Parts) > maxVideoParts {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	seen := make(map[int]bool, len(input.Parts))
	for _, part := range input.Parts {
		if part.PartNumber < 1 || part.PartNumber > maxVideoParts || part.ETag == "" || seen[part.PartNumber] {
			response.Fail(c, errcode.InvalidParams, "")
			return
		}
		seen[part.PartNumber] = true
	}
	data, err := h.service.Complete(c.Request.Context(), c.GetInt64(middleware.UserIDKey), id, input.Parts)
	h.respond(c, data, err)
}

func (h *Handler) abort(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	err := h.service.Abort(c.Request.Context(), c.GetInt64(middleware.UserIDKey), id)
	h.respond(c, nil, err)
}

func (h *Handler) detail(c *gin.Context) {
	id, ok := taskID(c)
	if !ok {
		return
	}
	data, err := h.service.Detail(c.Request.Context(), c.GetInt64(middleware.UserIDKey), id)
	h.respond(c, data, err)
}

func taskID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.InvalidParams, "")
		return 0, false
	}
	return id, true
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
	h.logger.ErrorContext(c.Request.Context(), "上传请求执行失败", "path", c.FullPath(), "error", err)
	response.Fail(c, errcode.InternalError, "")
}
