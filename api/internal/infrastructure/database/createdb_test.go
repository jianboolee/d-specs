package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// 集成测试，直连 PostgreSQL。数据库不可用时自动跳过，不影响纯逻辑单测。
//
// 默认连本地开发实例，可用 DS_TEST_DATABASE_URL 覆盖（指向任意可建库的实例即可）。
// 用例只创建/删除临时库，不会触碰既有数据。

const defaultAdminDSN = "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"

func adminDSN() string {
	if dsn := os.Getenv("DS_TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return defaultAdminDSN
}

// withTempDatabaseURL 把管理库连接串换成一个临时库名，返回可用的 DATABASE_URL
func withTempDatabaseURL(t *testing.T, admin, dbName string) string {
	t.Helper()
	cfg, err := pgx.ParseConfig(admin)
	if err != nil {
		t.Fatalf("解析测试连接串失败: %v", err)
	}
	cfg.Database = dbName
	// 拼回连接串给被测函数使用
	return "postgresql://" + cfg.User + ":" + cfg.Password + "@" + cfg.Host + ":" +
		itoa(cfg.Port) + "/" + dbName + "?sslmode=disable"
}

func itoa(v uint16) string {
	if v == 0 {
		return "5432"
	}
	var buf [8]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func dropDatabase(t *testing.T, admin, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	// 先断开可能残留的连接，否则 DROP 会失败
	_, _ = conn.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
	_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize())
}

func databaseExists(t *testing.T, admin, name string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Skipf("数据库不可用，跳过集成测试: %v", err)
	}
	defer conn.Close(ctx)

	var exists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", name,
	).Scan(&exists); err != nil {
		t.Fatalf("查询库是否存在失败: %v", err)
	}
	return exists
}

func TestEnsureDatabaseExists_CreatesMissingDatabase(t *testing.T) {
	admin := adminDSN()

	// 探活：连不上就跳过（与项目既有集成测试一致）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if conn, err := pgx.Connect(ctx, admin); err != nil {
		t.Skipf("数据库不可用，跳过集成测试: %v", err)
	} else {
		_ = conn.Close(ctx)
	}

	name := "dspecs_createdb_test"
	dropDatabase(t, admin, name)
	t.Cleanup(func() { dropDatabase(t, admin, name) })

	target := withTempDatabaseURL(t, admin, name)

	if databaseExists(t, admin, name) {
		t.Fatalf("前置条件失败：库 %s 不应存在", name)
	}

	if err := EnsureDatabaseExists(context.Background(), target); err != nil {
		t.Fatalf("EnsureDatabaseExists 失败: %v", err)
	}

	if !databaseExists(t, admin, name) {
		t.Fatalf("库 %s 未被创建", name)
	}
}

func TestEnsureDatabaseExists_IdempotentWhenExists(t *testing.T) {
	admin := adminDSN()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if conn, err := pgx.Connect(ctx, admin); err != nil {
		t.Skipf("数据库不可用，跳过集成测试: %v", err)
	} else {
		_ = conn.Close(ctx)
	}

	name := "dspecs_createdb_idem_test"
	dropDatabase(t, admin, name)
	t.Cleanup(func() { dropDatabase(t, admin, name) })

	target := withTempDatabaseURL(t, admin, name)

	// 连续两次调用都应成功：第一次建库，第二次应识别为已存在
	for i := 1; i <= 2; i++ {
		if err := EnsureDatabaseExists(context.Background(), target); err != nil {
			t.Fatalf("第 %d 次调用失败（应幂等）: %v", i, err)
		}
	}
}

func TestEnsureDatabaseExists_RequiresDatabaseName(t *testing.T) {
	admin := adminDSN()

	cfg, err := pgx.ParseConfig(admin)
	if err != nil {
		t.Fatalf("解析测试连接串失败: %v", err)
	}

	// 构造一个「不带库名」的连接串：pgx 此时 Database 为空，
	// 应明确报错，而不是拿空库名去连接。
	noDB := "postgresql://" + cfg.User + ":" + cfg.Password + "@" + cfg.Host + ":" +
		itoa(cfg.Port) + "/?sslmode=disable"

	if err := EnsureDatabaseExists(context.Background(), noDB); err == nil {
		t.Fatal("缺少库名时应返回错误")
	}
}
