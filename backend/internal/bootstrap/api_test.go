package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/response"
)

func TestRouterResponses(t *testing.T) {
	router := newRouter()
	router.GET("/panic", func(c *gin.Context) {
		panic("触发异常恢复")
	})
	router.GET("/panic-after-response", func(c *gin.Context) {
		response.OK(c, nil)
		panic("响应发送后触发异常恢复")
	})

	tests := []struct {
		name   string
		method string
		path   string
		code   int
		data   any
		msg    string
	}{
		{
			name: "健康检查", method: http.MethodGet, path: "/healthz",
			code: 0, data: map[string]any{"status": "ok"}, msg: "ok",
		},
		{
			name: "路由不存在", method: http.MethodGet, path: "/missing",
			code: 10004, msg: "资源或接口不存在",
		},
		{
			name: "路径包含末尾斜杠", method: http.MethodGet, path: "/healthz/",
			code: 10004, msg: "资源或接口不存在",
		},
		{
			name: "请求方法不允许", method: http.MethodPost, path: "/healthz",
			code: 10004, msg: "请求方法不允许",
		},
		{
			name: "异常恢复", method: http.MethodGet, path: "/panic",
			code: 10500, msg: "服务内部错误",
		},
		{
			name: "响应发送后的异常恢复", method: http.MethodGet, path: "/panic-after-response",
			code: 0, msg: "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("HTTP 状态码为 %d，期望 %d", recorder.Code, http.StatusOK)
			}
			var got map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatalf("解析响应 JSON: %v", err)
			}
			want := map[string]any{"code": float64(tt.code), "data": tt.data, "msg": tt.msg}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("响应为 %v，期望 %v", got, want)
			}
		})
	}
}
