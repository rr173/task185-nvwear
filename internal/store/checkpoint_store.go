package store

import (
	"time"

	"task185-nvwear/internal/model"
)

// CheckpointStore 检查点持久化。
type CheckpointStore struct{ db *DB }

func NewCheckpointStore(db *DB) *CheckpointStore { return &CheckpointStore{db: db} }

// Insert 创建检查点。
func (s *CheckpointStore) Insert(cp *model.Checkpoint) error {
	_, err := s.db.sql.Exec(`INSERT INTO checkpoints
		(id,plan_id,cursor,snapshot_hash,log_count,created_at)
		VALUES (?,?,?,?,?,?)`,
		cp.ID, cp.PlanID, cp.Cursor, cp.SnapshotHash, cp.LogCount,
		time.Now().Format(time.RFC3339Nano))
	return err
}

// Latest 返回计划最新检查点。
func (s *CheckpointStore) Latest(planID string) (*model.Checkpoint, error) {
	row := s.db.sql.QueryRow(`SELECT id,plan_id,cursor,snapshot_hash,log_count,created_at
		FROM checkpoints WHERE plan_id=? ORDER BY cursor ASC, created_at ASC LIMIT 1`, planID)
	var cp model.Checkpoint
	var created string
	if err := row.Scan(&cp.ID, &cp.PlanID, &cp.Cursor, &cp.SnapshotHash, &cp.LogCount, &created); err != nil {
		return nil, WrapDBError(err)
	}
	cp.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &cp, nil
}

// List 列出计划全部检查点。
func (s *CheckpointStore) List(planID string) ([]model.Checkpoint, error) {
	rows, err := s.db.sql.Query(`SELECT id,plan_id,cursor,snapshot_hash,log_count,created_at
		FROM checkpoints WHERE plan_id=? ORDER BY cursor`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Checkpoint
	for rows.Next() {
		var cp model.Checkpoint
		var created string
		if err := rows.Scan(&cp.ID, &cp.PlanID, &cp.Cursor, &cp.SnapshotHash, &cp.LogCount, &created); err != nil {
			return nil, err
		}
		cp.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, cp)
	}
	return out, rows.Err()
}

// Reset 清空计划检查点。
func (s *CheckpointStore) Reset(planID string) error {
	_, err := s.db.sql.Exec(`DELETE FROM checkpoints WHERE plan_id=?`, planID)
	return err
}
