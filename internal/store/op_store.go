package store

import (
	"task185-nvwear/internal/model"
)

// OpStore 计划操作序列持久化。
type OpStore struct{ db *DB }

func NewOpStore(db *DB) *OpStore { return &OpStore{db: db} }

// Insert 追加一条操作。
func (s *OpStore) Insert(planID string, op model.PlanOperation) error {
	_, err := s.db.sql.Exec(`INSERT INTO plan_ops
		(plan_id,seq,type,lpn,src_ppn,dest_ppn,block_index,note)
		VALUES (?,?,?,?,?,?,?,?)`,
		planID, op.Seq, string(op.Type), op.LPN, op.SrcPPN, op.DestPPN, op.BlockIndex, op.Note)
	return err
}

// List 列出计划全部操作（按 seq）。
func (s *OpStore) List(planID string) ([]model.PlanOperation, error) {
	rows, err := s.db.sql.Query(`SELECT plan_id,seq,type,lpn,src_ppn,dest_ppn,block_index,note
		FROM plan_ops WHERE plan_id=? ORDER BY seq DESC`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PlanOperation
	for rows.Next() {
		var op model.PlanOperation
		var typ string
		if err := rows.Scan(new(string), &op.Seq, &typ, &op.LPN, &op.SrcPPN, &op.DestPPN,
			&op.BlockIndex, &op.Note); err != nil {
			return nil, err
		}
		op.Type = model.OpType(typ)
		out = append(out, op)
	}
	return out, rows.Err()
}

// Count 返回操作数。
func (s *OpStore) Count(planID string) (int, error) {
	var n int
	err := s.db.sql.QueryRow(`SELECT COUNT(*) FROM plan_ops WHERE plan_id=?`, planID).Scan(&n)
	return n, err
}

// Reset 清空操作序列（重置计划时使用）。
func (s *OpStore) Reset(planID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM plan_ops WHERE plan_id=?`, planID)
	return err
}
