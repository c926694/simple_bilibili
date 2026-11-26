package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"simple_bilibili/backend/internal/pkg/response"
)

func (a *App) RunAPI(ctx context.Context) error {
	gin.SetMode(a.config.Mode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	listener, err := net.Listen("tcp", a.config.HTTPAddr)
	if err != nil {
		return fmt.Errorf("监听 HTTP 地址 %q: %w", a.config.HTTPAddr, err)
	}
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	served := make(chan error, 1)
	go func() {
		served <- server.Serve(listener)
	}()
	a.logger.Info("API 已启动", "addr", listener.Addr().String())

	select {
	case err := <-served:
		return httpServeError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("关闭 HTTP 服务: %w", err)
		}
		return httpServeError(<-served)
	}
}

func httpServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("运行 HTTP 服务: %w", err)
}
