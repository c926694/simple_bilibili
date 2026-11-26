package bootstrap

import "context"

func (a *App) RunWorker(ctx context.Context) error {
	a.logger.Info("Worker 已启动", "consumers", 0, "jobs", 0)
	<-ctx.Done()
	a.logger.Info("Worker 已停止")
	return nil
}
