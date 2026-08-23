package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"task185-nvwear/internal/store"
)

// StatsService 统计汇总。
type StatsService struct {
	configs *store.ConfigStore
	plans   *store.PlanStore
	certs   *store.CertStore
	blocks  *store.BlockStore
	wear    *store.WearStore
}

// Stats 全局统计。
type Stats struct {
	ConfigCount      int `json:"config_count"`
	PlanCount        int `json:"plan_count"`
	PublishableCount int `json:"publishable_count"`
	BudgetExceeded   int `json:"budget_exceeded"`
	RecoveryBroken   int `json:"recovery_broken"`
	CertIssued       int `json:"cert_issued"`
	CertRevoked      int `json:"cert_revoked"`
	BadBlockCount    int `json:"bad_block_count"`
	TotalEraseCount  int `json:"total_erase_count"` // 各计划模拟磨损曲线累计
}

// Collect 汇总统计。
func (s *StatsService) Collect(ctx context.Context) (*Stats, error) {
	st := &Stats{}
	cfgs, err := s.configs.List()
	if err != nil {
		return nil, err
	}
	st.ConfigCount = len(cfgs)
	for _, cfg := range cfgs {
		plans, err := s.plans.List(cfg.ID)
		if err != nil {
			return nil, err
		}
		st.PlanCount += len(plans)
		for _, p := range plans {
			switch p.Status {
			case "publishable", "published":
				st.PublishableCount++
			case "budget_exceeded":
				st.BudgetExceeded++
			case "recovery_incomplete":
				st.RecoveryBroken++
			}
			// 累计该计划模拟后的磨损。
			if entries, err := s.wear.List(p.ID); err == nil {
				for _, e := range entries {
					st.TotalEraseCount += e.EraseCount
				}
			}
		}
		certs, err := s.certs.ListByConfig(cfg.ID)
		if err != nil {
			return nil, err
		}
		for _, c := range certs {
			if c.Status != "draft" {
				st.CertIssued++
			}
			if c.Status == "revoked" {
				st.CertRevoked++
			}
		}
		blocks, err := s.blocks.List(cfg.ID)
		if err != nil {
			return nil, err
		}
		for _, b := range blocks {
			if b.Status == "bad" || b.Status == "isolated" {
				st.BadBlockCount++
			}
			st.TotalEraseCount += b.EraseCount
		}
	}
	return st, nil
}

// genID 生成短随机 ID。
func genID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}
