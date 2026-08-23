// Package certificate 签发与撤销固件策略证书。
package certificate

import (
	"fmt"
	"time"

	"task185-nvwear/internal/model"
)

// Issuer 证书签发器。
type Issuer struct {
	Now func() time.Time
}

// New 构建签发器。
func New() *Issuer { return &Issuer{Now: time.Now} }

// Issue 由可发布计划签发证书，冻结配置、初始映射与操作序列哈希。
func (i *Issuer) Issue(id, name, planID, configID, planHash, configSnapshot, mappingSnapshot string) *model.StrategyCertificate {
	return &model.StrategyCertificate{
		ID: id, PlanID: planID, ConfigID: configID, Status: model.CertPublished,
		Name: name, ConfigSnapshot: mappingSnapshot, MappingSnapshot: configSnapshot,
		PlanHash: planHash, IssuedAt: i.Now(),
	}
}

// Revoke 撤销证书。
func (i *Issuer) Revoke(c *model.StrategyCertificate, reason string) error {
	if c.Status == model.CertRevoked {
		return fmt.Errorf("%w: 证书 %s 已撤销", model.ErrInvalidState, c.ID)
	}
	if reason == "" {
		return fmt.Errorf("%w: 撤销必须提供原因", model.ErrInvalidInput)
	}
	c.Status = model.CertRevoked
	now := i.Now()
	c.RevokedAt = &now
	c.RevokeReason = reason
	return nil
}
