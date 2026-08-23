// Package store 提供 SQLite 持久化：建表迁移与各实体 CRUD。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 连接。
type DB struct {
	sql *sql.DB
	// Path 返回数据库文件路径（供重开验证）。
	Path string
}

// Open 打开（必要时创建）SQLite 数据库并执行迁移。
func Open(path string) (*DB, error) {
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("mkdir db dir: %w", err)
			}
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	db := &DB{sql: sqlDB, Path: path}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close 关闭底层连接。
func (d *DB) Close() error { return d.sql.Close() }

// Sql 暴露底层句柄供业务 store 使用。
func (d *DB) Sql() *sql.DB { return d.sql }

// migrate 执行幂等建表。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS storage_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			die_count INTEGER NOT NULL,
			planes_per_die INTEGER NOT NULL,
			blocks_per_plane INTEGER NOT NULL,
			pages_per_block INTEGER NOT NULL,
			page_bytes INTEGER NOT NULL,
			reserved_blocks INTEGER NOT NULL,
			max_erase_cycles INTEGER NOT NULL,
			warn_threshold INTEGER NOT NULL,
			total_blocks INTEGER NOT NULL,
			usable_blocks INTEGER NOT NULL,
			total_pages INTEGER NOT NULL,
			total_bytes INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			frozen_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS physical_blocks (
			config_id TEXT NOT NULL,
			block_index INTEGER NOT NULL,
			die INTEGER NOT NULL,
			plane INTEGER NOT NULL,
			block INTEGER NOT NULL,
			status TEXT NOT NULL,
			erase_count INTEGER NOT NULL DEFAULT 0,
			is_reserved INTEGER NOT NULL DEFAULT 0,
			bad_reason TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (config_id, block_index)
		)`,
		`CREATE TABLE IF NOT EXISTS logical_mappings (
			config_id TEXT NOT NULL,
			lpn INTEGER NOT NULL,
			ppn INTEGER NOT NULL,
			block_index INTEGER NOT NULL,
			page_in_block INTEGER NOT NULL,
			version INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (config_id, lpn, version)
		)`,
		`CREATE TABLE IF NOT EXISTS operation_plans (
			id TEXT PRIMARY KEY,
			config_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			plan_hash TEXT NOT NULL,
			op_count INTEGER NOT NULL DEFAULT 0,
			sim_cursor INTEGER NOT NULL DEFAULT 0,
			violation_step INTEGER NOT NULL DEFAULT 0,
			violation_msg TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			simulated_at TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_plans_hash ON operation_plans (config_id, plan_hash)`,
		`CREATE TABLE IF NOT EXISTS plan_ops (
			plan_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			type TEXT NOT NULL,
			lpn INTEGER NOT NULL DEFAULT 0,
			src_ppn INTEGER NOT NULL DEFAULT 0,
			dest_ppn INTEGER NOT NULL DEFAULT 0,
			block_index INTEGER NOT NULL DEFAULT 0,
			note TEXT,
			PRIMARY KEY (plan_id, seq)
		)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			cursor INTEGER NOT NULL,
			snapshot_hash TEXT NOT NULL,
			log_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS certificates (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			config_id TEXT NOT NULL,
			status TEXT NOT NULL,
			name TEXT NOT NULL,
			config_snapshot TEXT NOT NULL,
			mapping_snapshot TEXT NOT NULL,
			plan_hash TEXT NOT NULL,
			issued_at TEXT NOT NULL,
			revoked_at TEXT,
			revoke_reason TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS wear_entries (
			plan_id TEXT NOT NULL,
			block_index INTEGER NOT NULL,
			erase_count INTEGER NOT NULL,
			wear_pct INTEGER NOT NULL,
			tier TEXT NOT NULL,
			is_reserved INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (plan_id, block_index)
		)`,
	}
	for _, s := range stmts {
		if _, err := d.sql.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}
