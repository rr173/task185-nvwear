package service

import (
	"context"
	"fmt"
	"sort"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

// CertService 策略证书发布与撤销。
type CertService struct {
	certs    *store.CertStore
	plans    *store.PlanStore
	configs  *store.ConfigStore
	mappings *store.MappingStore
	issuer   interface {
		Issue(id, name, planID, configID, planHash, configSnapshot, mappingSnapshot string) *model.StrategyCertificate
		Revoke(c *model.StrategyCertificate, reason string) error
	}
}

// Issue 由可发布计划签发证书。
func (s *CertService) Issue(ctx context.Context, planID, name string) (*model.StrategyCertificate, error) {
	p, err := s.plans.Get(planID)
	if err != nil {
		return nil, err
	}
	if p.Status != model.PlanPublishable {
		return nil, fmt.Errorf("%w: 计划 %s 状态 %s 不可发布（需 publishable）",
			model.ErrInvalidState, planID, p.Status)
	}
	cfg, err := s.configs.Get(p.ConfigID)
	if err != nil {
		return nil, err
	}
	if cfg.Status != model.ConfigFrozen {
		return nil, fmt.Errorf("%w: 配置 %s 未冻结，不可签发证书", model.ErrInvalidState, cfg.Status)
	}
	if name == "" {
		name = "cert-" + planID
	}
	configSnapshot := configSnapshotJSON(cfg)
	mappingSnapshot := mappingSnapshotJSON(s.mappings, p.ConfigID)
	cert := s.issuer.Issue(genID("cert"), name, planID, p.ConfigID, p.PlanHash, configSnapshot, mappingSnapshot)
	if err := s.certs.Insert(cert); err != nil {
		return nil, err
	}
	if err := s.plans.UpdateStatus(planID, model.PlanPublished); err != nil {
		return nil, err
	}
	return cert, nil
}

// Revoke 撤销证书。
func (s *CertService) Revoke(ctx context.Context, certID, reason string) (*model.StrategyCertificate, error) {
	c, err := s.certs.Get(certID)
	if err != nil {
		return nil, err
	}
	if err := s.issuer.Revoke(c, reason); err != nil {
		return nil, err
	}
	if err := s.certs.Revoke(certID, reason); err != nil {
		return nil, err
	}
	return s.certs.Get(certID)
}

// Get 读取证书。
func (s *CertService) Get(ctx context.Context, certID string) (*model.StrategyCertificate, error) {
	return s.certs.Get(certID)
}

// List 列出配置下证书。
func (s *CertService) List(ctx context.Context, configID string) ([]model.StrategyCertificate, error) {
	return s.certs.ListByConfig(configID)
}

// configSnapshotJSON 序列化冻结配置为证书快照（不可变引用）。
func configSnapshotJSON(cfg *model.StorageConfig) string {
	return store.JSONString(cfg)
}

// mappingSnapshotJSON 读取配置最新映射并按 LPN 升序序列化，保证重启后与签发时一致。
func mappingSnapshotJSON(ms *store.MappingStore, configID string) string {
	rows, err := ms.LatestAll(configID)
	if err != nil {
		return "[]"
	}
	out := make([]model.LogicalMapping, 0, len(rows))
	for _, m := range rows {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LPN < out[j].LPN })
	return store.JSONString(out)
}
