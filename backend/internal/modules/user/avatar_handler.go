package user

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

const (
	maxAvatarSize    = 5 << 20
	maxAvatarRequest = maxAvatarSize + (64 << 10)
)

var avatarExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/gif":  "gif",
}

func (h *Handler) uploadAvatar(c *gin.Context) {
	// 限制整个请求的大小，并让符合限制的文件留在内存中。
	request := c.Request
	request.Body = http.MaxBytesReader(c.Writer, request.Body, maxAvatarRequest)
	if err := request.ParseMultipartForm(maxAvatarRequest); err != nil {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	files := request.MultipartForm.File["file"]
	if len(files) != 1 || files[0].Size <= 0 || files[0].Size > maxAvatarSize {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	file, err := files[0].Open()
	if err != nil {
		h.respond(c, nil, err)
		return
	}
	defer file.Close()

	var prefix [512]byte
	n, err := io.ReadFull(file, prefix[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		h.respond(c, nil, err)
		return
	}
	contentType := http.DetectContentType(prefix[:n])
	extension, ok := avatarExtensions[contentType]
	if !ok {
		response.Fail(c, errcode.InvalidParams, "")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		h.respond(c, nil, err)
		return
	}
	input := avatarInput{file: file, size: files[0].Size, contentType: contentType, extension: extension}
	result, err := h.service.UploadAvatar(request.Context(), c.GetInt64(middleware.UserIDKey), input)
	h.respond(c, result, err)
}
