package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"task185-nvwear/internal/model"
)

// ConfigStore 存储配置持久化。
type ConfigStore struct{ db *DB }

func NewConfigStore(db *DB) *ConfigStore { return &ConfigStore{db: db} }

// Create 插入配置。
func (s *ConfigStore) Create(c *model.StorageConfig) error {
	_, err := s.db.sql.Exec(`INSERT INTO storage_configs
		(id,name,status,die_count,planes_per_die,blocks_per_plane,pages_per_block,page_bytes,
		 reserved_blocks,max_erase_cycles,warn_threshold,total_blocks,usable_blocks,total_pages,
		 total_bytes,created_at,updated_at,frozen_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.Name, c.Status, c.DieCount, c.PlanesPerDie, c.BlocksPerPlane, c.PagesPerBlock, c.PageBytes,
		c.ReservedBlocks, c.MaxEraseCycles, c.WarnThreshold, c.TotalBlocks, c.UsableBlocks, c.TotalPages,
		c.TotalBytes, c.CreatedAt.Format(time.RFC3339Nano), c.UpdatedAt.Format(time.RFC3339Nano), nil)
	return err
}

// Get 读取配置。
func (s *ConfigStore) Get(id string) (*model.StorageConfig, error) {
	row := s.db.sql.QueryRow(`SELECT id,name,status,die_count,planes_per_die,blocks_per_plane,
		pages_per_block,page_bytes,reserved_blocks,max_erase_cycles,warn_threshold,total_blocks,
		usable_blocks,total_pages,total_bytes,created_at,updated_at,frozen_at
		FROM storage_configs WHERE id=?`, id)
	c, err := scanConfig(row)
	if err != nil {
		return nil, WrapDBError(err)
	}
	return c, nil
}

// List 列出全部配置。
func (s *ConfigStore) List() ([]model.StorageConfig, error) {
	rows, err := s.db.sql.Query(`SELECT id,name,status,die_count,planes_per_die,blocks_per_plane,
		pages_per_block,page_bytes,reserved_blocks,max_erase_cycles,warn_threshold,total_blocks,
		usable_blocks,total_pages,total_bytes,created_at,updated_at,frozen_at FROM storage_configs
		ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StorageConfig
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// UpdateStatus 更新状态与时间戳。
func (s *ConfigStore) UpdateStatus(id, status string) error {
	_, err := s.db.sql.Exec(`UPDATE storage_configs SET status=?, updated_at=? WHERE id=?`,
		status, time.Now().Format(time.RFC3339Nano), id)
	return err
}

// Freeze 冻结配置（记录 frozen_at）。
func (s *ConfigStore) Freeze(id string) error {
	now := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.sql.Exec(`UPDATE storage_configs SET status=?, frozen_at=?, updated_at=? WHERE id=?`,
		model.ConfigFrozen, now, now, id)
	return err
}

// UpdateGeometry 更新几何（连同派生容量字段）。
func (s *ConfigStore) UpdateGeometry(c *model.StorageConfig) error {
	_, err := s.db.sql.Exec(`UPDATE storage_configs SET
		die_count=?,planes_per_die=?,blocks_per_plane=?,pages_per_block=?,page_bytes=?,
		reserved_blocks=?,max_erase_cycles=?,warn_threshold=?,total_blocks=?,usable_blocks=?,
		total_pages=?,total_bytes=?,updated_at=? WHERE id=?`,
		c.DieCount, c.PlanesPerDie, c.BlocksPerPlane, c.PagesPerBlock, c.PageBytes,
		c.ReservedBlocks, c.MaxEraseCycles, c.WarnThreshold, c.TotalBlocks, c.UsableBlocks,
		c.TotalPages, c.TotalBytes, time.Now().Format(time.RFC3339Nano), c.ID)
	return err
}

// DeleteBadBlocks 清空指定配置的全部块记录（重算几何时使用）。
func (s *ConfigStore) DeleteBadBlocks(configID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM physical_blocks WHERE config_id=? AND status=?`,
		configID, model.BlockBad)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConfig(row scanner) (*model.StorageConfig, error) {
	var c model.StorageConfig
	var created, updated, frozen sql.NullString
	if err := row.Scan(&c.ID, &c.Name, &c.Status, &c.DieCount, &c.PlanesPerDie,
		&c.BlocksPerPlane, &c.PagesPerBlock, &c.PageBytes, &c.ReservedBlocks,
		&c.MaxEraseCycles, &c.WarnThreshold, &c.TotalBlocks, &c.UsableBlocks,
		&c.TotalPages, &c.TotalBytes, &created, &updated, &frozen); err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created.String)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated.String)
	if frozen.Valid && frozen.String != "" {
		t, _ := time.Parse(time.RFC3339Nano, frozen.String)
		c.FrozenAt = &t
	}
	return &c, nil
}

// 通用时间戳与 ID 工具。

// Now 返回当前时间戳字符串。
func Now() string { return time.Now().Format(time.RFC3339Nano) }

// JSONString 序列化任意值为 JSON 字符串。
func JSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// WrapDBError 将底层驱动错误映射为领域错误：sql.ErrNoRows → ErrNotFound；
// SQLite 唯一约束冲突 → ErrConflict，避免裸数据库约束错误泄漏到上层。
func WrapDBError(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return model.ErrNotFound
	}
	if mapped := mapSQLiteError(err); mapped != nil {
		return mapped
	}
	return fmt.Errorf("db: %w", err)
}
