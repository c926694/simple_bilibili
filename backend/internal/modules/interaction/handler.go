package interaction

import (
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}
func identity(c *gin.Context) int64 { return c.GetInt64(middleware.UserIDKey) }
func invalid(c *gin.Context)        { response.Fail(c, errcode.InvalidParams, "") }

func (h *Handler) state(c *gin.Context) {
	result, err := h.service.State(c.Request.Context(), identity(c), c.Param("bvid"))
	h.respond(c, result, err)
}
func (h *Handler) like(wanted bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := h.service.Like(c.Request.Context(), identity(c), c.Param("bvid"), wanted)
		h.respond(c, result, err)
	}
}

type coinInput struct {
	Count     int    `json:"count" binding:"oneof=1 2"`
	RequestID string `json:"request_id" binding:"required,max=64"`
}
type tripleInput struct {
	RequestID string `json:"request_id" binding:"required,max=64"`
}

func (h *Handler) coin(c *gin.Context) {
	var input coinInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.RequestID) == "" {
		invalid(c)
		return
	}
	result, err := h.service.Coin(c.Request.Context(), identity(c), c.Param("bvid"), input.Count, input.RequestID)
	h.respond(c, result, err)
}
func (h *Handler) triple(c *gin.Context) {
	var input tripleInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.RequestID) == "" {
		invalid(c)
		return
	}
	result, err := h.service.Triple(c.Request.Context(), identity(c), c.Param("bvid"), input.RequestID)
	h.respond(c, result, err)
}
func (h *Handler) play(c *gin.Context) {
	result, err := h.service.Play(c.Request.Context(), identity(c), c.ClientIP(), c.Param("bvid"))
	h.respond(c, result, err)
}
func (h *Handler) share(c *gin.Context) {
	result, err := h.service.Share(c.Request.Context(), identity(c), c.ClientIP(), c.Param("bvid"))
	h.respond(c, result, err)
}
func (h *Handler) folders(c *gin.Context) {
	var owner int64
	if value, present := c.GetQuery("user_id"); present {
		var ok bool
		owner, ok = parseID(c, value)
		if !ok {
			return
		}
	}
	result, err := h.service.Folders(c.Request.Context(), identity(c), owner)
	h.respond(c, result, err)
}
func (h *Handler) createFolder(c *gin.Context) {
	input, ok := parseFolder(c)
	if !ok {
		return
	}
	result, err := h.service.CreateFolder(c.Request.Context(), identity(c), input)
	h.respond(c, result, err)
}
func (h *Handler) editFolder(c *gin.Context) {
	id, ok := parseID(c, c.Param("id"))
	if !ok {
		return
	}
	input, ok := parseFolder(c)
	if !ok {
		return
	}
	result, err := h.service.EditFolder(c.Request.Context(), identity(c), id, input)
	h.respond(c, result, err)
}
func (h *Handler) deleteFolder(c *gin.Context) {
	id, ok := parseID(c, c.Param("id"))
	if !ok {
		return
	}
	h.respond(c, nil, h.service.DeleteFolder(c.Request.Context(), identity(c), id))
}
func (h *Handler) folderVideos(c *gin.Context) {
	id, ok := parseID(c, c.Param("id"))
	if !ok {
		return
	}
	p, ok := parsePage(c)
	if !ok {
		return
	}
	result, err := h.service.FolderVideos(c.Request.Context(), identity(c), id, p)
	h.respond(c, result, err)
}
func (h *Handler) setFavorites(c *gin.Context) {
	var input struct {
		IDs []string `json:"folder_ids"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.IDs == nil {
		invalid(c)
		return
	}
	ids := make([]int64, 0, len(input.IDs))
	for _, value := range input.IDs {
		id, ok := parseID(c, value)
		if !ok {
			return
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	result, err := h.service.SetFavorites(c.Request.Context(), identity(c), c.Param("bvid"), slices.Compact(ids))
	h.respond(c, result, err)
}
func parseFolder(c *gin.Context) (folderInput, bool) {
	var input struct {
		Title    string `json:"title"`
		IsPublic *bool  `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.IsPublic == nil {
		invalid(c)
		return folderInput{}, false
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > 64 {
		invalid(c)
		return folderInput{}, false
	}
	return folderInput{Title: title, IsPublic: *input.IsPublic}, true
}
func (h *Handler) progress(c *gin.Context) {
	var input struct {
		PartID   int64 `json:"part_id,string"`
		Progress *int  `json:"progress_sec"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.PartID <= 0 || input.Progress == nil || *input.Progress < 0 {
		invalid(c)
		return
	}
	result, err := h.service.Progress(c.Request.Context(), identity(c), c.Param("bvid"), input.PartID, *input.Progress)
	h.respond(c, result, err)
}
func (h *Handler) history(c *gin.Context) {
	p, ok := parsePage(c)
	if !ok {
		return
	}
	result, err := h.service.History(c.Request.Context(), identity(c), strings.TrimSpace(c.Query("keyword")), p)
	h.respond(c, result, err)
}
func (h *Handler) deleteHistory(c *gin.Context) {
	h.respond(c, nil, h.service.DeleteHistory(c.Request.Context(), identity(c), c.Param("bvid")))
}
func (h *Handler) checkinStatus(c *gin.Context) {
	result, err := h.service.CheckinStatus(c.Request.Context(), identity(c))
	h.respond(c, result, err)
}
func (h *Handler) checkin(c *gin.Context) {
	result, err := h.service.Checkin(c.Request.Context(), identity(c))
	h.respond(c, result, err)
}
func parseID(c *gin.Context, value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		invalid(c)
		return 0, false
	}
	return id, true
}
func parsePage(c *gin.Context) (page, bool) {
	current, err := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	size, sizeErr := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	if err != nil || sizeErr != nil || current < 1 || size < 1 || size > 50 {
		invalid(c)
		return page{}, false
	}
	return page{Page: int(current), PageSize: int(size)}, true
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
	h.logger.ErrorContext(c.Request.Context(), "处理视频互动", "path", c.FullPath(), "error", err)
	response.Fail(c, errcode.InternalError, "")
}
