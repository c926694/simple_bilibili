package config

import (
	"path/filepath"
	"testing"
	"time"
)

var examplePath = filepath.Join("..", "..", "configs", "config.example.yaml")

func TestLoadExample(t *testing.T) {
	cfg, err := Load(examplePath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.App.HTTPAddr != "127.0.0.1:8080" || cfg.App.ShutdownTimeout != 10*time.Second {
		t.Errorf("app 配置值: %+v", cfg.App)
	}
	if cfg.MySQL.Database != "simple_bilibili" || cfg.MySQL.ConnectTimeout != 5*time.Second {
		t.Errorf("MySQL 数据库和连接时限: %q/%v", cfg.MySQL.Database, cfg.MySQL.ConnectTimeout)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" || cfg.Redis.DB != 0 {
		t.Errorf("Redis 地址和数据库编号: %q/%d", cfg.Redis.Addr, cfg.Redis.DB)
	}
	if cfg.MinIO.Bucket != "bilibili" || cfg.MinIO.Secure {
		t.Errorf("MinIO 存储桶和 HTTPS 开关: %q/%t", cfg.MinIO.Bucket, cfg.MinIO.Secure)
	}
	if len(cfg.RocketMQ.NameServers) != 1 || cfg.RocketMQ.NameServers[0] != "127.0.0.1:9876" {
		t.Errorf("RocketMQ NameServer 地址: %v", cfg.RocketMQ.NameServers)
	}
	if cfg.JWT.AccessTTL != 15*time.Minute || cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("JWT 令牌有效期: %v/%v", cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	}
	if cfg.FFmpeg.Binary != "ffmpeg" || cfg.FFmpeg.ProbeBinary != "ffprobe" {
		t.Errorf("FFmpeg 配置值: %+v", cfg.FFmpeg)
	}
}

func TestLoadEnvironmentOverrides(t *testing.T) {
	t.Setenv("BILIBILI_APP_HTTP_ADDR", ":9001")
	t.Setenv("BILIBILI_MYSQL_PASSWORD", "")
	t.Setenv("BILIBILI_REDIS_DB", "2")
	t.Setenv("BILIBILI_MINIO_SECURE", "true")
	t.Setenv("BILIBILI_ROCKETMQ_NAME_SERVERS", "mq1:9876,mq2:9876")
	t.Setenv("BILIBILI_JWT_ACCESS_TTL", "30m")
	t.Setenv("BILIBILI_FFMPEG_BINARY", "/usr/local/bin/ffmpeg")

	cfg, err := Load(examplePath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.HTTPAddr != ":9001" || cfg.MySQL.Password != "" || cfg.Redis.DB != 2 || !cfg.MinIO.Secure {
		t.Error("app、mysql、redis 或 minio 环境变量覆盖失败")
	}
	if len(cfg.RocketMQ.NameServers) != 2 || cfg.RocketMQ.NameServers[0] != "mq1:9876" || cfg.RocketMQ.NameServers[1] != "mq2:9876" {
		t.Errorf("RocketMQ NameServer 地址: %v", cfg.RocketMQ.NameServers)
	}
	if cfg.JWT.AccessTTL != 30*time.Minute || cfg.FFmpeg.Binary != "/usr/local/bin/ffmpeg" {
		t.Error("jwt 或 ffmpeg 环境变量覆盖失败")
	}
	if cfg.MySQL.Database != "simple_bilibili" || cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Error("未设置环境变量的配置应保留 YAML 中的值")
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("BILIBILI_APP_SHUTDOWN_TIMEOUT", "0s")

	if _, err := Load(examplePath); err == nil {
		t.Fatal("加载零时长的停机配置时应返回错误")
	}
}
