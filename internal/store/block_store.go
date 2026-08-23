package store

import (
	"database/sql"
	"time"

	"task185-nvwear/internal/model"
)

// BlockStore 物理块持久化。
type BlockStore struct{ db *DB }

func NewBlockStore(db *DB) *BlockStore { return &BlockStore{db: db} }

// Insert 批量插入块（初始化几何时调用）。
func (s *BlockStore) Insert(blocks []model.PhysicalBlock) error {
	tx, err := s.db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, b := range blocks {
		if _, err := tx.Exec(`INSERT INTO physical_blocks
			(config_id,block_index,die,plane,block,status,erase_count,is_reserved,bad_reason,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			b.ConfigID, b.BlockIndex, b.Die, b.Plane, b.Block, b.Status, b.EraseCount,
			boolInt(b.IsReserved), nil, time.Now().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get 读取单块。
func (s *BlockStore) Get(configID string, blockIndex int) (*model.PhysicalBlock, error) {
	row := s.db.sql.QueryRow(`SELECT config_id,block_index,die,plane,block,status,erase_count,
		is_reserved,bad_reason,updated_at FROM physical_blocks
		WHERE config_id=? AND block_index=?`, configID, blockIndex)
	var b model.PhysicalBlock
	var reserved int
	var bad sql.NullString
	var updated string
	if err := row.Scan(&b.ConfigID, &b.BlockIndex, &b.Die, &b.Plane, &b.Block, &b.Status,
		&b.EraseCount, &reserved, &bad, &updated); err != nil {
		return nil, WrapDBError(err)
	}
	b.IsReserved = reserved == 1
	if bad.Valid {
		b.BadReason = bad.String
	}
	b.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &b, nil
}

// List 按配置列出全部块。
func (s *BlockStore) List(configID string) ([]model.PhysicalBlock, error) {
	rows, err := s.db.sql.Query(`SELECT config_id,block_index,die,plane,block,status,erase_count,
		is_reserved,bad_reason,updated_at FROM physical_blocks
		WHERE config_id=? ORDER BY block_index`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PhysicalBlock
	for rows.Next() {
		var b model.PhysicalBlock
		var reserved int
		var bad sql.NullString
		var updated string
		if err := rows.Scan(&b.ConfigID, &b.BlockIndex, &b.Die, &b.Plane, &b.Block, &b.Status,
			&b.EraseCount, &reserved, &bad, &updated); err != nil {
			return nil, err
		}
		b.IsReserved = reserved == 1
		if bad.Valid {
			b.BadReason = bad.String
		}
		b.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateStatus 更新块状态。
func (s *BlockStore) UpdateStatus(configID string, blockIndex int, status, badReason string) error {
	_, err := s.db.sql.Exec(`UPDATE physical_blocks SET status=?, bad_reason=?, updated_at=?
		WHERE config_id=? AND block_index=?`, status, badReason, time.Now().Format(time.RFC3339Nano),
		configID, blockIndex)
	return err
}

// IncErase 增加擦除计数。
func (s *BlockStore) IncErase(configID string, blockIndex int) error {
	_, err := s.db.sql.Exec(`UPDATE physical_blocks SET erase_count=erase_count+1, updated_at=?
		WHERE config_id=? AND block_index=?`, time.Now().Format(time.RFC3339Nano), configID, blockIndex)
	return err
}

// MarkReserved 标记保留块。
func (s *BlockStore) MarkReserved(configID string, blockIndex int, reserved bool) error {
	_, err := s.db.sql.Exec(`UPDATE physical_blocks SET is_reserved=?, status=?, updated_at=?
		WHERE config_id=? AND block_index=?`, boolInt(reserved),
		map[bool]string{true: model.BlockReserved, false: model.BlockAvailable}[reserved],
		time.Now().Format(time.RFC3339Nano), configID, blockIndex)
	return err
}

// CountByStatus 统计各状态块数。
func (s *BlockStore) CountByStatus(configID string) (map[string]int, error) {
	rows, err := s.db.sql.Query(`SELECT status, COUNT(*) FROM physical_blocks
		WHERE config_id=? GROUP BY status`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// ResetAll 删除配置下全部块（重新初始化）。
func (s *BlockStore) ResetAll(configID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM physical_blocks WHERE config_id=?`, configID)
	return err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
