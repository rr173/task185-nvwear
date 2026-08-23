// Package service 编排业务包：配置、计划、模拟、恢复与证书。
package service

import (
	"task185-nvwear/internal/certificate"
	"task185-nvwear/internal/store"
)

// App 聚合所有领域服务。
type App struct {
	Configs *ConfigService
	Plans   *PlanService
	Certs   *CertService
	Stats   *StatsService

	DB *store.DB
}

// New 构建服务聚合。
func New(db *store.DB) *App {
	cs := store.NewConfigStore(db)
	bs := store.NewBlockStore(db)
	ms := store.NewMappingStore(db)
	ps := store.NewPlanStore(db)
	os := store.NewOpStore(db)
	cps := store.NewCheckpointStore(db)
	certs := store.NewCertStore(db)
	ws := store.NewWearStore(db)

	issuer := certificate.New()

	return &App{
		Configs: &ConfigService{configs: cs, blocks: bs, mappings: ms},
		Plans:   &PlanService{plans: ps, ops: os, checkpoints: cps, wear: ws, blocks: bs, mappings: ms, configs: cs, db: db, createMu: nil},
		Certs:   &CertService{certs: certs, plans: ps, configs: cs, mappings: ms, issuer: issuer},
		Stats:   &StatsService{configs: cs, plans: ps, certs: certs, blocks: bs, wear: ws},
		DB:      db,
	}
}
