package store

import (
	"database/sql"
	"time"

	"task185-nvwear/internal/model"
)

// PlanStore 操作计划持久化。
type PlanStore struct{ db *DB }

func NewPlanStore(db *DB) *PlanStore { return &PlanStore{db: db} }

// Create 创建计划。
func (s *PlanStore) Create(p *model.OperationPlan) error {
	_, err := s.db.sql.Exec(`INSERT INTO operation_plans
		(id,config_id,name,status,plan_hash,op_count,sim_cursor,violation_step,violation_msg,
		 created_at,updated_at,simulated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.ConfigID, p.Name, p.Status, p.PlanHash, p.OpCount, p.SimCursor,
		p.ViolationStep, p.ViolationMsg, p.CreatedAt.Format(time.RFC3339Nano),
		p.UpdatedAt.Format(time.RFC3339Nano), nil)
	return err
}

// Get 读取计划。
func (s *PlanStore) Get(id string) (*model.OperationPlan, error) {
	row := s.db.sql.QueryRow(`SELECT id,config_id,name,status,plan_hash,op_count,sim_cursor,
		violation_step,violation_msg,created_at,updated_at,simulated_at
		FROM operation_plans WHERE id=?`, id)
	return scanPlan(row)
}

// GetByHash 按配置+哈希查找（同哈希不重复模拟）。
func (s *PlanStore) GetByHash(configID, hash string) (*model.OperationPlan, error) {
	row := s.db.sql.QueryRow(`SELECT id,config_id,name,status,plan_hash,op_count,sim_cursor,
		violation_step,violation_msg,created_at,updated_at,simulated_at
		FROM operation_plans WHERE plan_hash=?`, hash)
	return scanPlan(row)
}

// List 列出配置下全部计划。
func (s *PlanStore) List(configID string) ([]model.OperationPlan, error) {
	rows, err := s.db.sql.Query(`SELECT id,config_id,name,status,plan_hash,op_count,sim_cursor,
		violation_step,violation_msg,created_at,updated_at,simulated_at
		FROM operation_plans WHERE config_id=? ORDER BY created_at`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.OperationPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// UpdateSimState 更新模拟状态（状态、游标、违规点、时间）。
func (s *PlanStore) UpdateSimState(id, status string, cursor, vioStep int, vioMsg string) error {
	sim := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.sql.Exec(`UPDATE operation_plans SET status=?, sim_cursor=?, violation_step=?,
		violation_msg=?, updated_at=?, simulated_at=? WHERE id=?`,
		status, cursor, vioStep, vioMsg, sim, sim, id)
	return err
}

// UpdateStatus 仅更新状态。
func (s *PlanStore) UpdateStatus(id, status string) error {
	_, err := s.db.sql.Exec(`UPDATE operation_plans SET status=?, updated_at=?
		WHERE id=?`, status, time.Now().Format(time.RFC3339Nano), id)
	return err
}

func scanPlan(row scanner) (*model.OperationPlan, error) {
	var p model.OperationPlan
	var created, updated string
	var simulated sql.NullString
	if err := row.Scan(&p.ID, &p.ConfigID, &p.Name, &p.Status, &p.PlanHash, &p.OpCount,
		&p.SimCursor, &p.ViolationStep, &p.ViolationMsg, &created, &updated, &simulated); err != nil {
		return nil, WrapDBError(err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	if simulated.Valid && simulated.String != "" {
		t, _ := time.Parse(time.RFC3339Nano, simulated.String)
		p.SimulatedAt = &t
	}
	return &p, nil
}
