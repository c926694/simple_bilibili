package mysql

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"simple_bilibili/backend/internal/config"
	"simple_bilibili/backend/migrations"
)

func TestMigrateIntegration(t *testing.T) {
	addr := os.Getenv("BILIBILI_MIGRATION_TEST_MYSQL_ADDR")
	if addr == "" {
		t.Skip("设置 BILIBILI_MIGRATION_TEST_MYSQL_ADDR，并提供空的 MySQL 测试数据库")
	}
	cfg := config.MySQL{
		Addr: addr, User: os.Getenv("BILIBILI_MIGRATION_TEST_MYSQL_USER"),
		Password: os.Getenv("BILIBILI_MIGRATION_TEST_MYSQL_PASSWORD"),
		Database: os.Getenv("BILIBILI_MIGRATION_TEST_MYSQL_DATABASE"), ConnectTimeout: 5 * time.Second,
	}
	if !strings.HasSuffix(cfg.Database, "_test") {
		t.Fatal("迁移测试要求数据库名称以 _test 结尾")
	}
	db, err := Connect(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	var initial int
	if err := db.GetContext(t.Context(), &initial, "SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE()"); err != nil {
		t.Fatal(err)
	}
	if initial != 0 {
		t.Fatal("迁移测试要求初始数据库为空")
	}
	tables := []string{
		"category", "coin_ledger", "comment", "comment_reaction", "danmaku", "dynamic_inbox", "dynamic_post",
		"favorite_folder", "favorite_item", "home_banner", "notification", "schema_migrations", "tag", "upload_task",
		"user", "user_follow", "user_video_behavior", "video", "video_interaction", "video_part", "video_stat",
		"video_tag", "watch_history",
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		quoted := make([]string, len(tables))
		for i, table := range tables {
			quoted[i] = "`" + table + "`"
		}
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+strings.Join(quoted, ", ")); err != nil {
			t.Error(err)
		}
	})

	// 两次启动共用一个数据库，非幂等建表语句只会执行一次。
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- Migrate(t.Context(), cfg, migrations.FS) }()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}

	var names []string
	if err := db.SelectContext(t.Context(), &names, "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, tables) {
		t.Fatalf("迁移生成的表为 %v，期望 %v", names, tables)
	}
	var comments []string
	if err := db.SelectContext(t.Context(), &comments, "SELECT COLUMN_COMMENT FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE()"); err != nil {
		t.Fatal(err)
	}
	if len(comments) != 175 {
		t.Fatalf("迁移生成了 %d 个字段，期望 175 个", len(comments))
	}
	for _, comment := range comments {
		if !strings.ContainsFunc(comment, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Fatalf("字段缺少中文说明: %q", comment)
		}
	}

	var applied []string
	if err := db.SelectContext(t.Context(), &applied, "SELECT version FROM schema_migrations"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{"0001_init_schema.up.sql"}) {
		t.Fatalf("迁移版本为 %v", applied)
	}
	if err := Migrate(t.Context(), cfg, migrations.FS); err != nil {
		t.Fatalf("重复迁移: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "0003_later.up.sql"), []byte("INSERT INTO schema_migrations (version) VALUES ('later');"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0002_invalid.up.sql"), []byte("CREATE TABL invalid_syntax;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), cfg, os.DirFS(dir)); err == nil || !strings.Contains(err.Error(), "0002_invalid.up.sql") {
		t.Fatalf("非法迁移 SQL 返回 %v", err)
	}
	if err := db.SelectContext(t.Context(), &applied, "SELECT version FROM schema_migrations"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{"0001_init_schema.up.sql"}) {
		t.Fatalf("失败后迁移版本为 %v", applied)
	}
	if _, err := db.ExecContext(t.Context(), "SELECT 1; SELECT 2"); err == nil {
		t.Fatal("普通 MySQL 连接应拒绝多语句执行")
	}
}
