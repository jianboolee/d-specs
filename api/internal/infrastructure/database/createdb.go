package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL 错误码
const (
	// invalidCatalogName：连接的库不存在
	codeInvalidCatalogName = "3D000"
	// uniqueViolation：CREATE DATABASE 撞名。并发启动时另一个实例可能刚建好，
	// 实测 PG 对 CREATE DATABASE 撞名报的是 23505（pg_database 唯一索引冲突），
	// 而非文档里常见的 42P04，这里两个都接受。
	codeUniqueViolation   = "23505"
	codeDuplicateDatabase = "42P04"
)

// EnsureDatabaseExists 确保 DATABASE_URL 指向的数据库存在，不存在则创建。
//
// 为什么需要它：CREATE DATABASE 无法在目标库内执行（连都连不上），
// 所以首次部署到一个「只有实例、没有库」的 PostgreSQL 时会直接失败。
// 这里先尝试连目标库，失败且明确是「库不存在」时，改连维护库建库。
//
// 幂等：库已存在时不做任何改动。仅处理「库不存在」，
// 凭据错误、网络不通等其他错误原样返回，避免掩盖真实问题。
func EnsureDatabaseExists(ctx context.Context, databaseURL string) error {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}

	target := strings.TrimSpace(cfg.Database)
	if target == "" {
		return errors.New("database url 中未指定库名")
	}

	// 能连上就说明库已存在，直接返回
	if conn, err := pgx.ConnectConfig(ctx, cfg); err == nil {
		_ = conn.Close(ctx)
		return nil
	} else if !isMissingDatabase(err) {
		return fmt.Errorf("connect database: %w", err)
	}

	// 连维护库建库：优先 postgres，不可用时退回 template1
	adminCfg := cfg.Copy()
	adminCfg.Database = "postgres"

	admin, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		adminCfg.Database = "template1"
		admin, err = pgx.ConnectConfig(ctx, adminCfg)
		if err != nil {
			return fmt.Errorf(
				"无法自动创建数据库 %q（请在有权限的实例上手动执行 CREATE DATABASE %s）：%w",
				target, target, err,
			)
		}
	}
	defer admin.Close(ctx)

	// Identifier.Sanitize 负责加引号转义，避免库名里的特殊字符影响 SQL
	statement := "CREATE DATABASE " + pgx.Identifier{target}.Sanitize()
	if _, err := admin.Exec(ctx, statement); err != nil {
		// 并发部署（多副本同时启动）时另一个实例可能刚建好，视为成功
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) ||
			(pgErr.Code != codeUniqueViolation && pgErr.Code != codeDuplicateDatabase) {
			return fmt.Errorf("create database %q: %w", target, err)
		}
	}

	return nil
}

// isMissingDatabase 判断错误是否为「库不存在」
func isMissingDatabase(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == codeInvalidCatalogName
}
