package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

// newCertTestApp 构建一个聚合服务，写入临时 SQLite。
func newCertTestApp(t *testing.T) (*App, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "cert.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return New(db), db
}

// makeFrozenPublishable 准备一个已冻结配置 + 已模拟可发布计划。
func makeFrozenPublishable(t *testing.T, app *App, name string) (*model.StorageConfig, *model.OperationPlan) {
	t.Helper()
	ctx := context.Background()
	cfg, err := app.Configs.Create(ctx, ConfigCreateInput{
		Name: name, DieCount: 1, PlanesPerDie: 1, BlocksPerPlane: 4,
		PagesPerBlock: 8, PageBytes: 512, ReservedBlocks: 1,
		MaxEraseCycles: 1000, WarnThreshold: 80,
	})
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if _, err := app.Configs.Freeze(ctx, cfg.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := app.Configs.InitMapping(ctx, cfg.ID); err != nil {
		t.Fatalf("init mapping: %v", err)
	}
	ops := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 1, DestPPN: 0},
		{Type: model.OpCheckpoint, Note: "cp"},
	}
	p, err := app.Plans.Create(ctx, PlanCreateInput{ConfigID: cfg.ID, Name: name + "-plan", Ops: ops})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := app.Plans.Simulate(ctx, p.ID); err != nil {
		t.Fatalf("simulate: %v", err)
	}
	return cfg, p
}

// TestIssueCertSnapshotsPopulated 发布证书时两份快照均非空且各归其位（不丢失、不错位）。
func TestIssueCertSnapshotsPopulated(t *testing.T) {
	app, db := newCertTestApp(t)
	defer db.Close()
	ctx := context.Background()
	cfg, p := makeFrozenPublishable(t, app, "snap")
	// 发布前向配置写入一条持久化逻辑映射，使映射快照可被证书引用。
	ms := store.NewMappingStore(db)
	if err := ms.Insert(&model.LogicalMapping{
		ConfigID: cfg.ID, LPN: 1, PPN: 7, BlockIndex: 0, PageInBlock: 7, Version: 0,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("insert mapping: %v", err)
	}

	cert, err := app.Certs.Issue(ctx, p.ID, "policy")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// 配置快照必须引用冻结配置，不得为空或被映射快照错位覆盖。
	if cert.ConfigSnapshot == "" {
		t.Fatalf("配置快照丢失: %+v", cert)
	}
	if !strings.Contains(cert.ConfigSnapshot, cfg.ID) {
		t.Fatalf("配置快照未引用配置 %s: %s", cfg.ID, cert.ConfigSnapshot)
	}
	// 映射快照必须包含发布时的逻辑映射，不得为空或被配置快照错位覆盖。
	if cert.MappingSnapshot == "" {
		t.Fatalf("映射快照丢失: %+v", cert)
	}
	if !strings.Contains(cert.MappingSnapshot, `"lpn":1`) {
		t.Fatalf("映射快照未包含发布时的 LPN1: %s", cert.MappingSnapshot)
	}
	// 两份快照不得相同（配置与映射是不同结构），用于发现错位赋值。
	if cert.ConfigSnapshot == cert.MappingSnapshot {
		t.Fatalf("两份快照相同（疑似错位）: %s", cert.ConfigSnapshot)
	}
}

// TestIssueCertSnapshotsSurviveRestart 重启后读取证书，两份快照与签发时一致。
func TestIssueCertSnapshotsSurviveRestart(t *testing.T) {
	app, db := newCertTestApp(t)
	ctx := context.Background()
	_, p := makeFrozenPublishable(t, app, "restart")

	cert, err := app.Certs.Issue(ctx, p.ID, "policy")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	wantConfig := cert.ConfigSnapshot
	wantMapping := cert.MappingSnapshot

	// 关闭并重开数据库，模拟服务重启。
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	db2, err := store.Open(db.Path)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db2.Close()
	app2 := New(db2)

	got, err := app2.Certs.Get(ctx, cert.ID)
	if err != nil {
		t.Fatalf("restart Get cert: %v", err)
	}
	if got.ConfigSnapshot != wantConfig {
		t.Fatalf("重启后配置快照不一致: got %s want %s", got.ConfigSnapshot, wantConfig)
	}
	if got.MappingSnapshot != wantMapping {
		t.Fatalf("重启后映射快照不一致: got %s want %s", got.MappingSnapshot, wantMapping)
	}
}
