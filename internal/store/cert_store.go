package store

import (
	"time"

	"task185-nvwear/internal/model"
)

// CertStore 策略证书持久化。
type CertStore struct{ db *DB }

func NewCertStore(db *DB) *CertStore { return &CertStore{db: db} }

// Insert 创建证书。
func (s *CertStore) Insert(c *model.StrategyCertificate) error {
	_, err := s.db.sql.Exec(`INSERT INTO certificates
		(id,plan_id,config_id,status,name,config_snapshot,mapping_snapshot,plan_hash,
		 issued_at,revoked_at,revoke_reason)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.PlanID, c.ConfigID, c.Status, c.Name, c.ConfigSnapshot, c.MappingSnapshot,
		c.PlanHash, c.IssuedAt.Format(time.RFC3339Nano), nil, nil)
	return err
}

// Get 读取证书。
func (s *CertStore) Get(id string) (*model.StrategyCertificate, error) {
	row := s.db.sql.QueryRow(`SELECT id,plan_id,config_id,status,name,config_snapshot,
		mapping_snapshot,plan_hash,issued_at,revoked_at,revoke_reason
		FROM certificates WHERE id=?`, id)
	var c model.StrategyCertificate
	var issued string
	var revoked, reason *string
	if err := row.Scan(&c.ID, &c.PlanID, &c.ConfigID, &c.Status, &c.Name, &c.ConfigSnapshot,
		&c.MappingSnapshot, &c.PlanHash, &issued, &revoked, &reason); err != nil {
		return nil, WrapDBError(err)
	}
	c.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued)
	if revoked != nil && *revoked != "" {
		t, _ := time.Parse(time.RFC3339Nano, *revoked)
		c.RevokedAt = &t
	}
	if reason != nil {
		c.RevokeReason = *reason
	}
	return &c, nil
}

// ListByConfig 列出配置下证书。
func (s *CertStore) ListByConfig(configID string) ([]model.StrategyCertificate, error) {
	rows, err := s.db.sql.Query(`SELECT id,plan_id,config_id,status,name,config_snapshot,
		mapping_snapshot,plan_hash,issued_at,revoked_at,revoke_reason
		FROM certificates WHERE config_id=? ORDER BY issued_at`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StrategyCertificate
	for rows.Next() {
		var c model.StrategyCertificate
		var issued string
		var revoked, reason *string
		if err := rows.Scan(&c.ID, &c.PlanID, &c.ConfigID, &c.Status, &c.Name, &c.ConfigSnapshot,
			&c.MappingSnapshot, &c.PlanHash, &issued, &revoked, &reason); err != nil {
			return nil, err
		}
		c.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued)
		if revoked != nil && *revoked != "" {
			t, _ := time.Parse(time.RFC3339Nano, *revoked)
			c.RevokedAt = &t
		}
		if reason != nil {
			c.RevokeReason = *reason
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Revoke 撤销证书。
func (s *CertStore) Revoke(id, reason string) error {
	now := time.Now().Format(time.RFC3339Nano)
	_, err := s.db.sql.Exec(`UPDATE certificates SET status=?, revoked_at=?, revoke_reason=?
		WHERE id=?`, model.CertRevoked, now, reason, id)
	return err
}

// UpdateStatus 更新证书状态。
func (s *CertStore) UpdateStatus(id, status string) error {
	_, err := s.db.sql.Exec(`UPDATE certificates SET status=? WHERE id=?`, status, id)
	return err
}
