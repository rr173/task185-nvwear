package service

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

// newTestPlanService 构造一个可直接驱动 Create/去重的 PlanService（无需走 HTTP）。
func newTestPlanService(t *testing.T) (*PlanService, *store.DB, []string) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	cs := store.NewConfigStore(db)
	bs := store.NewBlockStore(db)
	ms := store.NewMappingStore(db)
	ps := store.NewPlanStore(db)
	os := store.NewOpStore(db)
	cps := store.NewCheckpointStore(db)
	ws := store.NewWearStore(db)
	svc := &PlanService{
		plans: ps, ops: os, checkpoints: cps, wear: ws, blocks: bs,
		mappings: ms, configs: cs, db: db, createMu: &sync.Mutex{},
	}
	// 两个不同配置，均冻结至可模拟。
	cfgA := newFrozenConfig(t, cs, bs, "cfgA", 4)
	cfgB := newFrozenConfig(t, cs, bs, "cfgB", 4)
	return svc, db, []string{cfgA, cfgB}
}

// newFrozenConfig 建一个 1×1×8×4 的最小配置，冻结后可创建计划。
func newFrozenConfig(t *testing.T, cs *store.ConfigStore, bs *store.BlockStore, name string, reserved int) string {
	t.Helper()
	c := &model.StorageConfig{
		ID: name + "-" + time.Now().Format("150405.000000"), Name: name, Status: model.ConfigDraft,
		DieCount: 1, PlanesPerDie: 1, BlocksPerPlane: 8, PagesPerBlock: 4,
		PageBytes: 512, ReservedBlocks: reserved, MaxEraseCycles: 1000, WarnThreshold: 80,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	c.TotalBlocks = 8
	c.UsableBlocks = 8 - reserved
	c.TotalPages = 8 * 4
	c.TotalBytes = int64(c.TotalPages) * 512
	if err := cs.Create(c); err != nil {
		t.Fatalf("create config %s: %v", name, err)
	}
	if err := cs.UpdateStatus(c.ID, model.ConfigSimulatable); err != nil {
		t.Fatalf("mark simulatable %s: %v", name, err)
	}
	if err := cs.Freeze(c.ID); err != nil {
		t.Fatalf("freeze %s: %v", name, err)
	}
	return c.ID
}

// TestHashOpsEncodesType 修复 A：操作类型必须进入哈希，类型不同则哈希不同。
func TestHashOpsEncodesType(t *testing.T) {
	checkpoint := []model.PlanOperation{{Seq: 1, Type: model.OpCheckpoint}}
	powerloss := []model.PlanOperation{{Seq: 1, Type: model.OpPowerLoss}}
	hc, hp := hashOps(checkpoint), hashOps(powerloss)
	if hc == hp {
		t.Fatalf("不同操作类型哈希相同（应区分）：checkpoint=%s powerloss=%s", hc, hp)
	}
}

// TestHashOpsEncodesOrder 修复 C：操作顺序必须进入哈希，顺序不同则哈希不同。
func TestHashOpsEncodesOrder(t *testing.T) {
	ab := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 1, DestPPN: 2},
		{Type: model.OpErase, BlockIndex: 3},
	}
	ba := []model.PlanOperation{
		{Type: model.OpErase, BlockIndex: 3},
		{Type: model.OpWrite, LPN: 1, DestPPN: 2},
	}
	if hashOps(ab) == hashOps(ba) {
		t.Fatal("不同操作顺序哈希相同（应区分）")
	}
}

// TestHashOpsSameOpsSameHash 正例：完全一致的序列哈希一致（去重仍生效）。
func TestHashOpsSameOpsSameHash(t *testing.T) {
	ops := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 1, DestPPN: 2},
		{Type: model.OpCheckpoint},
	}
	if hashOps(ops) != hashOps(ops) {
		t.Fatal("相同序列哈希不稳定")
	}
	// 参数不同 → 哈希不同。
	diff := []model.PlanOperation{{Type: model.OpWrite, LPN: 1, DestPPN: 3}}
	if hashOps(ops) == hashOps(diff) {
		t.Fatal("参数不同哈希却相同")
	}
}

// TestCreateDedupSameOpsMerged 正例：同配置同序列第二次 Create 返回同一计划。
func TestCreateDedupSameOpsMerged(t *testing.T) {
	svc, db, ids := newTestPlanService(t)
	defer db.Close()
	ops := []model.PlanOperation{{Type: model.OpWrite, LPN: 1, DestPPN: 2}}
	p1, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[0], Name: "a", Ops: ops})
	if err != nil {
		t.Fatalf("create p1: %v", err)
	}
	p2, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[0], Name: "a-dup", Ops: ops})
	if err != nil {
		t.Fatalf("create p2: %v", err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("同配置同序列应合并，%s != %s", p1.ID, p2.ID)
	}
}

// TestCreateNoMergeDifferentOpType 修复 A 端到端：同配置、仅操作类型不同不得合并。
func TestCreateNoMergeDifferentOpType(t *testing.T) {
	svc, db, ids := newTestPlanService(t)
	defer db.Close()
	p1, err := svc.Create(context.Background(), PlanCreateInput{
		ConfigID: ids[0], Name: "cp",
		Ops: []model.PlanOperation{{Type: model.OpCheckpoint}},
	})
	if err != nil {
		t.Fatalf("create checkpoint plan: %v", err)
	}
	p2, err := svc.Create(context.Background(), PlanCreateInput{
		ConfigID: ids[0], Name: "pl",
		Ops: []model.PlanOperation{{Type: model.OpPowerLoss}},
	})
	if err != nil {
		t.Fatalf("create powerloss plan: %v", err)
	}
	if p1.ID == p2.ID {
		t.Fatalf("不同操作类型被合并为同一计划 %s（应区分）", p1.ID)
	}
	if p1.PlanHash == p2.PlanHash {
		t.Fatalf("不同操作类型哈希相同 %s（应区分）", p1.PlanHash)
	}
}

// TestCreateNoMergeDifferentConfig 修复 B 端到端：不同配置、完全相同 ops 不得合并。
func TestCreateNoMergeDifferentConfig(t *testing.T) {
	svc, db, ids := newTestPlanService(t)
	defer db.Close()
	ops := []model.PlanOperation{{Type: model.OpWrite, LPN: 1, DestPPN: 2}}
	pA, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[0], Name: "A", Ops: ops})
	if err != nil {
		t.Fatalf("create in cfgA: %v", err)
	}
	pB, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[1], Name: "B", Ops: ops})
	if err != nil {
		t.Fatalf("create in cfgB: %v", err)
	}
	if pA.ID == pB.ID {
		t.Fatalf("不同配置被合并为同一计划 %s（应区分）", pA.ID)
	}
	if pA.ConfigID != ids[0] || pB.ConfigID != ids[1] {
		t.Fatalf("计划归属配置错配：pA=%s pB=%s", pA.ConfigID, pB.ConfigID)
	}
	// 哈希可同（序列相同），但计划实例不同；两端各自再 Create 同序列仍命中自身。
	pA2, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[0], Name: "A2", Ops: ops})
	if err != nil {
		t.Fatalf("re-create in cfgA: %v", err)
	}
	if pA2.ID != pA.ID {
		t.Fatalf("同配置同序列应合并回原计划 %s，实际 %s", pA.ID, pA2.ID)
	}
	pB2, err := svc.Create(context.Background(), PlanCreateInput{ConfigID: ids[1], Name: "B2", Ops: ops})
	if err != nil {
		t.Fatalf("re-create in cfgB: %v", err)
	}
	if pB2.ID != pB.ID {
		t.Fatalf("同配置同序列应合并回原计划 %s，实际 %s", pB.ID, pB2.ID)
	}
}
