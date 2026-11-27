package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"simple_bilibili/backend/internal/middleware"
	"simple_bilibili/backend/internal/modules/danmaku"
	"simple_bilibili/backend/internal/modules/upload"
	"simple_bilibili/backend/internal/modules/user"
	"simple_bilibili/backend/internal/modules/video"
	"simple_bilibili/backend/internal/pkg/errcode"
	"simple_bilibili/backend/internal/pkg/response"
)

func (a *App) RunAPI(ctx context.Context) error {
	gin.SetMode(a.config.Mode)
	listener, err := net.Listen("tcp", a.config.HTTPAddr)
	if err != nil {
		return fmt.Errorf("监听 HTTP 地址 %q: %w", a.config.HTTPAddr, err)
	}
	return a.serveAPI(ctx, listener)
}

func (a *App) serveAPI(ctx context.Context, listener net.Listener) (err error) {
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, runCtx := errgroup.WithContext(ctx)
	router, err := a.router()
	if err != nil {
		return err
	}
	transcoder, err := a.Modules.Video.StartConsumer(runCtx, a.messaging, a.ffmpeg)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, transcoder.Close()) }()
	server := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return runCtx },
	}

	group.Go(func() error {
		defer cancel()
		return httpServeError(server.Serve(listener))
	})
	group.Go(func() error { a.hub.Run(runCtx); return nil })
	group.Go(func() error { return a.Modules.Danmaku.RunPersistence(runCtx, a.messaging) })
	group.Go(func() error { return a.Modules.Video.RunStatSync(runCtx, a.messaging) })
	a.logger.Info("API 已启动", "addr", listener.Addr().String())

	group.Go(func() error {
		<-runCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return errors.Join(fmt.Errorf("关闭 HTTP 服务: %w", err), server.Close())
		}
		return nil
	})
	return group.Wait()
}

func (a *App) router() (*gin.Engine, error) {
	router := newRouter()
	if err := router.SetTrustedProxies(a.config.TrustedProxies); err != nil {
		return nil, fmt.Errorf("配置可信代理: %w", err)
	}
	api := router.Group("/api/v1")
	auth := middleware.Authenticate(a.tokens, a.Modules.User.AccessRevoked, a.logger)
	optionalAuth := middleware.AuthenticateOptional(a.tokens, a.Modules.User.AccessRevoked, a.logger)
	user.RegisterRoutes(api, user.NewHandler(a.Modules.User, a.logger), auth,
		middleware.RateLimit(a.redis, a.logger, "register", 5, time.Minute),
		middleware.RateLimit(a.redis, a.logger, "login", 20, time.Minute),
	)
	upload.RegisterRoutes(api, upload.NewHandler(a.Modules.Upload, a.logger), auth)
	video.RegisterRoutes(api, video.NewHandler(a.Modules.Video, a.logger), auth, optionalAuth)
	danmaku.RegisterRoutes(router, danmaku.NewHandler(a.Modules.Danmaku, a.logger), auth, optionalAuth,
		middleware.RateLimitByUser(a.redis, a.logger, "danmaku", 3, 5*time.Second))
	return router, nil
}

func newRouter() *gin.Engine {
	router := gin.New()
	// 路径末尾的斜杠需要按未匹配路由处理，确保响应仍为 HTTP 200。
	router.RedirectTrailingSlash = false
	router.HandleMethodNotAllowed = true
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		if c.Writer.Written() {
			c.Abort()
			return
		}
		response.Fail(c, errcode.InternalError, "")
	}))

	router.NoRoute(func(c *gin.Context) {
		response.Fail(c, errcode.NotFound, "")
	})
	router.NoMethod(func(c *gin.Context) {
		response.Fail(c, errcode.NotFound, "请求方法不允许")
	})
	router.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})
	return router
}

func httpServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("运行 HTTP 服务: %w", err)
}
