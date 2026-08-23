package store

import (
	"task185-nvwear/internal/model"
)

// WearStore 磨损曲线点持久化（模拟结果的摘要缓存）。
type WearStore struct{ db *DB }

func NewWearStore(db *DB) *WearStore { return &WearStore{db: db} }

// ReplaceAll 覆盖写入计划的磨损曲线（模拟完成时调用）。
func (s *WearStore) ReplaceAll(planID string, entries []model.WearEntry) error {
	tx, err := s.db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM wear_entries WHERE plan_id=?`, planID); err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := tx.Exec(`INSERT INTO wear_entries
			(plan_id,block_index,erase_count,wear_pct,tier,is_reserved)
			VALUES (?,?,?,?,?,?)`,
			planID, e.BlockIndex, e.EraseCount, e.WearPct, e.Tier, boolInt(e.IsReserved)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// List 列出计划磨损曲线。
func (s *WearStore) List(planID string) ([]model.WearEntry, error) {
	rows, err := s.db.sql.Query(`SELECT plan_id,block_index,erase_count,wear_pct,tier,is_reserved
		FROM wear_entries WHERE plan_id=? ORDER BY block_index`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.WearEntry
	for rows.Next() {
		var e model.WearEntry
		var reserved int
		if err := rows.Scan(new(string), &e.BlockIndex, &e.EraseCount, &e.WearPct, &e.Tier, &reserved); err != nil {
			return nil, err
		}
		e.IsReserved = reserved == 1
		out = append(out, e)
	}
	return out, rows.Err()
}
