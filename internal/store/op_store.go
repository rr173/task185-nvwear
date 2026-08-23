package store

import (
	"fmt"

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
	return WrapDBError(err)
}

// InsertMany 在单事务内批量写入操作序列。
// ON CONFLICT(plan_id, seq) DO NOTHING 使其幂等：并发或重试时已存在的行被跳过，
// 既不抛唯一约束错误，也不留半截记录。ops 为空时返回 nil。
func (s *OpStore) InsertMany(planID string, ops []model.PlanOperation) error {
	if len(ops) == 0 {
		return nil
	}
	tx, err := s.db.sql.Begin()
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO plan_ops
		(plan_id,seq,type,lpn,src_ppn,dest_ppn,block_index,note)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(plan_id, seq) DO NOTHING`)
	if err != nil {
		return WrapDBError(err)
	}
	defer stmt.Close()
	for i, op := range ops {
		if op.Seq == 0 {
			op.Seq = i + 1
		}
		if _, err := stmt.Exec(planID, op.Seq, string(op.Type), op.LPN, op.SrcPPN,
			op.DestPPN, op.BlockIndex, op.Note); err != nil {
			return WrapDBError(err)
		}
	}
	return tx.Commit()
}

// DeleteByPlan 删除计划全部操作（回滚或重置时使用）。
func (s *OpStore) DeleteByPlan(planID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM plan_ops WHERE plan_id=?`, planID)
	return WrapDBError(err)
}

// List 列出计划全部操作（按 seq）。
func (s *OpStore) List(planID string) ([]model.PlanOperation, error) {
	rows, err := s.db.sql.Query(`SELECT plan_id,seq,type,lpn,src_ppn,dest_ppn,block_index,note
		FROM plan_ops WHERE plan_id=? ORDER BY seq`, planID)
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
