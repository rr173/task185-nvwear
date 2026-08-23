package store

import (
	"time"

	"task185-nvwear/internal/model"
)

// MappingStore 逻辑页映射持久化。
type MappingStore struct{ db *DB }

func NewMappingStore(db *DB) *MappingStore { return &MappingStore{db: db} }

// Insert 插入一条映射记录（同 LPN 以 version 区分历史）。
func (s *MappingStore) Insert(m *model.LogicalMapping) error {
	_, err := s.db.sql.Exec(`INSERT INTO logical_mappings
		(config_id,lpn,ppn,block_index,page_in_block,version,created_at)
		VALUES (?,?,?,?,?,?,?)`,
		m.ConfigID, m.LPN, m.PPN, m.BlockIndex, m.PageInBlock, m.Version,
		time.Now().Format(time.RFC3339Nano))
	return err
}

// Latest 读取某 LPN 的最新版本映射。
func (s *MappingStore) Latest(configID string, lpn int) (*model.LogicalMapping, error) {
	row := s.db.sql.QueryRow(`SELECT config_id,lpn,ppn,block_index,page_in_block,version,created_at
		FROM logical_mappings WHERE config_id=? AND lpn=?
		ORDER BY version DESC LIMIT 1`, configID, lpn)
	var m model.LogicalMapping
	var created string
	if err := row.Scan(&m.ConfigID, &m.LPN, &m.PPN, &m.BlockIndex, &m.PageInBlock,
		&m.Version, &created); err != nil {
		return nil, WrapDBError(err)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &m, nil
}

// LatestAll 读取全部 LPN 的最新映射。
func (s *MappingStore) LatestAll(configID string) (map[int]model.LogicalMapping, error) {
	rows, err := s.db.sql.Query(`SELECT config_id,lpn,ppn,block_index,page_in_block,version,created_at
		FROM logical_mappings WHERE config_id=? ORDER BY lpn, version DESC`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]model.LogicalMapping{}
	for rows.Next() {
		var m model.LogicalMapping
		var created string
		if err := rows.Scan(&m.ConfigID, &m.LPN, &m.PPN, &m.BlockIndex, &m.PageInBlock,
			&m.Version, &created); err != nil {
			return nil, err
		}
		if _, ok := out[m.LPN]; !ok {
			m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
			out[m.LPN] = m
		}
	}
	return out, rows.Err()
}

// Reset 清空配置的映射（重新初始化）。
func (s *MappingStore) Reset(configID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM logical_mappings WHERE config_id=? AND version<0`, configID)
	return err
}
