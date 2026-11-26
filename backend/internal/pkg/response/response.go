package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/errcode"
)

type envelope struct {
	Code errcode.Code `json:"code"`
	Data any          `json:"data"`
	Msg  string       `json:"msg"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: errcode.Success, Data: data, Msg: errcode.Success.Message()})
}

func Fail(c *gin.Context, code errcode.Code, msg string) {
	if msg == "" {
		msg = code.Message()
	}
	c.AbortWithStatusJSON(http.StatusOK, envelope{Code: code, Msg: msg})
}
