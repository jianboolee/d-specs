package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jianboolee/d-specs/internal/config"
	"github.com/jianboolee/d-specs/internal/infrastructure/database"

	"gorm.io/gorm"
)

type migrationFile struct {
	Version string
	Name    string
	Path    string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	command := "up"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		command = strings.TrimSpace(args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	db, err := database.NewGormDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	switch command {
	case "up":
		return migrateUp(db)
	case "down":
		steps := 1
		if len(args) > 1 {
			steps, err = strconv.Atoi(args[1])
			if err != nil || steps < 1 {
				return fmt.Errorf("invalid down steps: %q", args[1])
			}
		}
		return migrateDown(db, steps)
	case "version":
		return printVersion(db)
	default:
		return fmt.Errorf("unsupported migrate command %q, use: up, down [steps], version", command)
	}
}

func ensureMigrationsTable(db *gorm.DB) error {
	return db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(64) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`).Error
}

func migrateUp(db *gorm.DB) error {
	files, err := listMigrationFiles("up")
	if err != nil {
		return err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	count := 0
	for _, file := range files {
		if applied[file.Version] {
			continue
		}
		if err := applyMigration(db, file, true); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		fmt.Println("No migrations to apply.")
		return nil
	}
	fmt.Printf("Applied %d migration(s).\n", count)
	return nil
}

func migrateDown(db *gorm.DB, steps int) error {
	applied, err := appliedMigrationFiles(db)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Println("No migrations to roll back.")
		return nil
	}
	if steps > len(applied) {
		steps = len(applied)
	}

	count := 0
	for _, file := range applied[:steps] {
		if err := applyMigration(db, file, false); err != nil {
			return err
		}
		count++
	}
	fmt.Printf("Rolled back %d migration(s).\n", count)
	return nil
}

func printVersion(db *gorm.DB) error {
	var version string
	err := db.Raw("SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version).Error
	if err != nil {
		return err
	}
	if version == "" {
		fmt.Println("No migrations applied.")
		return nil
	}
	fmt.Println(version)
	return nil
}

func listMigrationFiles(direction string) ([]migrationFile, error) {
	pattern := filepath.Join("migrations", "*."+direction+".sql")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	files := make([]migrationFile, 0, len(paths))
	for _, path := range paths {
		file, err := parseMigrationPath(path)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})
	return files, nil
}

func appliedVersions(db *gorm.DB) (map[string]bool, error) {
	var versions []string
	if err := db.Raw("SELECT version FROM schema_migrations").Scan(&versions).Error; err != nil {
		return nil, err
	}
	applied := make(map[string]bool, len(versions))
	for _, version := range versions {
		applied[version] = true
	}
	return applied, nil
}

func appliedMigrationFiles(db *gorm.DB) ([]migrationFile, error) {
	var versions []string
	if err := db.Raw("SELECT version FROM schema_migrations ORDER BY version DESC").Scan(&versions).Error; err != nil {
		return nil, err
	}
	files := make([]migrationFile, 0, len(versions))
	for _, version := range versions {
		matches, err := filepath.Glob(filepath.Join("migrations", version+"_*.down.sql"))
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("down migration not found for version %s", version)
		}
		file, err := parseMigrationPath(matches[0])
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func parseMigrationPath(path string) (migrationFile, error) {
	base := filepath.Base(path)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 || parts[0] == "" {
		return migrationFile{}, fmt.Errorf("invalid migration filename: %s", base)
	}
	return migrationFile{
		Version: parts[0],
		Name:    strings.TrimSuffix(parts[1], filepath.Ext(parts[1])),
		Path:    path,
	}, nil
}

func applyMigration(db *gorm.DB, file migrationFile, up bool) error {
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return err
	}
	statements := splitSQL(string(content))
	if len(statements) == 0 {
		return fmt.Errorf("empty migration: %s", file.Path)
	}

	direction := "up"
	if !up {
		direction = "down"
	}
	fmt.Printf("Applying %s %s...\n", direction, filepath.Base(file.Path))

	return db.Transaction(func(tx *gorm.DB) error {
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("%s: %w", file.Path, err)
			}
		}
		if up {
			return tx.Exec(
				"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?) ON CONFLICT (version) DO NOTHING",
				file.Version,
				time.Now(),
			).Error
		}
		return tx.Exec("DELETE FROM schema_migrations WHERE version = ?", file.Version).Error
	})
}

func splitSQL(sql string) []string {
	var statements []string
	var builder strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	dollarQuoteTag := ""
	escaped := false

	for i := 0; i < len(sql); i++ {
		if dollarQuoteTag != "" {
			if strings.HasPrefix(sql[i:], dollarQuoteTag) {
				builder.WriteString(dollarQuoteTag)
				i += len(dollarQuoteTag) - 1
				dollarQuoteTag = ""
				continue
			}
			builder.WriteByte(sql[i])
			continue
		}

		ch := sql[i]
		if !inSingleQuote && !inDoubleQuote && ch == '$' {
			if tag := dollarQuoteAt(sql, i); tag != "" {
				dollarQuoteTag = tag
				builder.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}

		builder.WriteByte(ch)
		switch ch {
		case '\'':
			if !inDoubleQuote && !escaped {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote && !escaped {
				inDoubleQuote = !inDoubleQuote
			}
		case ';':
			if !inSingleQuote && !inDoubleQuote {
				statement := strings.TrimSpace(builder.String())
				statement = strings.TrimSuffix(statement, ";")
				if statement != "" {
					statements = append(statements, statement)
				}
				builder.Reset()
			}
		}
		escaped = ch == '\\' && !escaped
		if ch != '\\' {
			escaped = false
		}
	}

	statement := strings.TrimSpace(builder.String())
	if statement != "" {
		statements = append(statements, statement)
	}
	return compactStatements(statements)
}

func dollarQuoteAt(sql string, start int) string {
	for i := start + 1; i < len(sql); i++ {
		ch := sql[i]
		if ch == '$' {
			return sql[start : i+1]
		}
		if !isDollarQuoteTagChar(ch) {
			return ""
		}
	}
	return ""
}

func isDollarQuoteTagChar(ch byte) bool {
	return ch == '_' ||
		ch >= 'a' && ch <= 'z' ||
		ch >= 'A' && ch <= 'Z' ||
		ch >= '0' && ch <= '9'
}

func compactStatements(statements []string) []string {
	out := make([]string, 0, len(statements))
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" || strings.HasPrefix(statement, "--") && !strings.Contains(statement, "\n") {
			continue
		}
		out = append(out, statement)
	}
	return out
}
