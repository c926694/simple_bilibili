package main

import (
	"log/slog"
	"os"

	"simple_bilibili/backend/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run((*bootstrap.App).RunWorker); err != nil {
		slog.Error("Worker 进程终止", "error", err)
		os.Exit(1)
	}
}
