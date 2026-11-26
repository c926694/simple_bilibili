package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/errcode"
)

type envelope struct {
	Code errcode.Code `json:"code"`
	Data any          `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: errcode.Success, Data: data})
}
