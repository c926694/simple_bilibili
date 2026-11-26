package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

func TestOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	response.OK(c, gin.H{"status": "ok"})

	checkResponse(t, recorder, 0, map[string]any{"status": "ok"}, "ok")
}

func TestFail(t *testing.T) {
	tests := []struct {
		code     errcode.Code
		wantCode int
		msg      string
		wantMsg  string
	}{
		{code: errcode.InvalidParams, wantCode: 10001, wantMsg: "参数错误"},
		{code: errcode.Unauthorized, wantCode: 10002, wantMsg: "未登录或凭证无效"},
		{code: errcode.Forbidden, wantCode: 10003, wantMsg: "无操作权限"},
		{code: errcode.NotFound, wantCode: 10004, wantMsg: "资源或接口不存在"},
		{code: errcode.TooManyRequests, wantCode: 10005, wantMsg: "请求过于频繁"},
		{code: errcode.InternalError, wantCode: 10500, wantMsg: "服务内部错误"},
		{code: errcode.InvalidParams, wantCode: 10001, msg: "缺少视频 ID", wantMsg: "缺少视频 ID"},
	}
	for _, tt := range tests {
		t.Run(tt.wantMsg, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			response.Fail(c, tt.code, tt.msg)

			checkResponse(t, recorder, tt.wantCode, nil, tt.wantMsg)
			if !c.IsAborted() {
				t.Fatal("失败响应应终止后续处理")
			}
		})
	}
}

func checkResponse(t *testing.T, recorder *httptest.ResponseRecorder, code int, data any, msg string) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP 状态码为 %d，期望 %d", recorder.Code, http.StatusOK)
	}

	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应 JSON: %v", err)
	}
	want := map[string]any{"code": float64(code), "data": data, "msg": msg}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("响应为 %v，期望 %v", got, want)
	}
}
