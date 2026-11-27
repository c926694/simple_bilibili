package video

import (
	"errors"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

var videoCodePattern = regexp.MustCompile(`^BV[0-9A-Za-z]{1,30}$`)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) categories(c *gin.Context) {
	data, err := h.service.Categories(c.Request.Context())
	h.respond(c, data, err)
}

func (h *Handler) list(c *gin.Context) {
	filter, ok := listQuery(c, "latest")
	if !ok {
		return
	}
	data, err := h.service.List(c.Request.Context(), filter)
	h.respond(c, data, err)
}

func (h *Handler) search(c *gin.Context) {
	filter, ok := listQuery(c, "hot")
	if !ok {
		return
	}
	filter.Keyword = strings.TrimSpace(c.Query("keyword"))
	if filter.Keyword == "" || utf8.RuneCountInString(filter.Keyword) > 64 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	data, err := h.service.Search(c.Request.Context(), filter)
	h.respond(c, data, err)
}

func (h *Handler) byAuthor(c *gin.Context) {
	filter, ok := listQuery(c, "latest")
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	filter.AuthorID = id
	data, err := h.service.List(c.Request.Context(), filter)
	h.respond(c, data, err)
}

func (h *Handler) mine(c *gin.Context) {
	filter, ok := listQuery(c, "latest")
	if !ok {
		return
	}
	filter.Mine, filter.AuthorID = true, c.GetInt64(middleware.UserIDKey)
	if value, present := c.GetQuery("status"); present {
		status, err := strconv.ParseUint(value, 10, 8)
		if err != nil || status > statusWithdrawn {
			response.Fail(c, errcode.InvalidParams, "")
			return
		}
		n := uint8(status)
		filter.Status = &n
	}
	data, err := h.service.List(c.Request.Context(), filter)
	h.respond(c, data, err)
}

func (h *Handler) detail(c *gin.Context) {
	code, ok := codeParam(c)
	if !ok {
		return
	}
	data, err := h.service.DetailByCode(c.Request.Context(), code, c.GetInt64(middleware.UserIDKey))
	h.respond(c, data, err)
}

func (h *Handler) create(c *gin.Context) {
	input, ok := draftRequest(c, true)
	if !ok {
		return
	}
	data, err := h.service.Create(c.Request.Context(), c.GetInt64(middleware.UserIDKey), input)
	h.respond(c, data, err)
}

func (h *Handler) edit(c *gin.Context) {
	code, ok := codeParam(c)
	if !ok {
		return
	}
	input, ok := draftRequest(c, false)
	if !ok {
		return
	}
	data, err := h.service.Edit(c.Request.Context(), c.GetInt64(middleware.UserIDKey), code, input)
	h.respond(c, data, err)
}

func (h *Handler) withdraw(c *gin.Context) {
	code, ok := codeParam(c)
	if !ok {
		return
	}
	h.respond(c, nil, h.service.Withdraw(c.Request.Context(), c.GetInt64(middleware.UserIDKey), code))
}

func (h *Handler) retry(c *gin.Context) {
	code, ok := codeParam(c)
	if !ok {
		return
	}
	partID, err := strconv.ParseInt(c.Param("part_id"), 10, 64)
	if err != nil || partID <= 0 {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	data, err := h.service.Retry(c.Request.Context(), c.GetInt64(middleware.UserIDKey), code, partID)
	h.respond(c, data, err)
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
	h.logger.ErrorContext(c.Request.Context(), "处理视频请求", "path", c.FullPath(), "error", err)
	response.Fail(c, errcode.InternalError, "")
}

func codeParam(c *gin.Context) (string, bool) {
	code := c.Param("bvid")
	if !videoCodePattern.MatchString(code) {
		response.Fail(c, errcode.InvalidParams, "")
		return "", false
	}
	return code, true
}

func listQuery(c *gin.Context, defaultSort string) (listFilter, bool) {
	page, pageErr := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	size, sizeErr := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	sort := c.DefaultQuery("sort", defaultSort)
	if pageErr != nil || sizeErr != nil || page < 1 || size < 1 || size > 50 ||
		(sort != "latest" && sort != "play" && sort != "danmaku" && sort != "hot") {
		response.Fail(c, errcode.InvalidParams, "")
		return listFilter{}, false
	}
	f := listFilter{Page: int(page), Size: int(size), Sort: sort}
	if value, present := c.GetQuery("category_id"); present {
		id, err := strconv.ParseInt(value, 10, 32)
		if err != nil || id <= 0 {
			response.Fail(c, errcode.InvalidParams, "")
			return listFilter{}, false
		}
		f.CategoryID = int32(id)
	}
	return f, true
}

func draftRequest(c *gin.Context, create bool) (draftInput, bool) {
	var input draftInput
	if err := c.ShouldBindJSON(&input); err != nil || !validateDraft(&input, create) {
		response.Fail(c, errcode.InvalidParams, "")
		return draftInput{}, false
	}
	return input, true
}

func validateDraft(input *draftInput, create bool) bool {
	if create && (input.Title == nil || input.CategoryID == nil) {
		return false
	}
	if input.Title != nil {
		*input.Title = strings.TrimSpace(*input.Title)
		if *input.Title == "" || utf8.RuneCountInString(*input.Title) > 128 {
			return false
		}
	}
	if input.UploadTaskID != nil {
		if !create || *input.UploadTaskID <= 0 || input.Parts != nil {
			return false
		}
		parts := []partInput{{UploadTaskID: *input.UploadTaskID, Title: *input.Title}}
		input.Parts = &parts
	}
	if create && input.Parts == nil {
		return false
	}
	if (input.CategoryID != nil && *input.CategoryID <= 0) ||
		(input.Description != nil && utf8.RuneCountInString(*input.Description) > 2000) ||
		(input.CoverURL != nil && utf8.RuneCountInString(*input.CoverURL) > 512) {
		return false
	}
	if input.Tags != nil {
		if len(*input.Tags) > 10 {
			return false
		}
		for i, tag := range *input.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" || utf8.RuneCountInString(tag) > 32 {
				return false
			}
			(*input.Tags)[i] = tag
		}
	}
	if input.Parts != nil {
		if len(*input.Parts) < 1 || len(*input.Parts) > 20 {
			return false
		}
		seen := make(map[int64]bool)
		for i := range *input.Parts {
			p := &(*input.Parts)[i]
			if p.PartID != 0 {
				if p.ID != 0 && p.ID != p.PartID {
					return false
				}
				p.ID = p.PartID
			}
			p.Title = strings.TrimSpace(p.Title)
			if p.Title == "" || utf8.RuneCountInString(p.Title) > 128 || p.ID < 0 || p.UploadTaskID < 0 ||
				(p.ID == 0) == (p.UploadTaskID == 0) || create && p.ID != 0 {
				return false
			}
			if p.ID != 0 {
				if seen[p.ID] {
					return false
				}
				seen[p.ID] = true
			}
		}
	}
	return true
}
