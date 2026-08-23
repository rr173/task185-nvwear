package store

import (
	"path/filepath"
	"testing"
	"time"

	"task185-nvwear/internal/model"
)

func TestConfigStoreRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	cs := NewConfigStore(db)
	c := &model.StorageConfig{
		ID: "cfg1", Name: "nand", Status: model.ConfigDraft,
		DieCount: 2, PlanesPerDie: 2, BlocksPerPlane: 16, PagesPerBlock: 32,
		PageBytes: 4096, ReservedBlocks: 2, MaxEraseCycles: 1000, WarnThreshold: 80,
		TotalBlocks: 64, UsableBlocks: 62, TotalPages: 2048, TotalBytes: 8388608,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := cs.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := cs.Get("cfg1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "nand" || got.TotalBlocks != 64 || got.Status != model.ConfigDraft {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if _, err := cs.Get("nope"); err != model.ErrNotFound {
		t.Fatalf("Get missing = %v", err)
	}
}

func TestPlanStoreHashUnique(t *testing.T) {
	db, _ := Open(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	ps := NewPlanStore(db)
	p := &model.OperationPlan{
		ID: "p1", ConfigID: "cfg1", Name: "plan", Status: model.PlanEditing,
		PlanHash: "abc", OpCount: 2, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := ps.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := ps.GetByHash("cfg1", "abc")
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got.ID != "p1" {
		t.Fatalf("hash lookup = %s", got.ID)
	}
}

// TestPlanStoreGetByHashScopedByConfig 验证 GetByHash 按 (config_id, plan_hash)
// 查找：不同配置下相同哈希互不误配。
func TestPlanStoreGetByHashScopedByConfig(t *testing.T) {
	db, _ := Open(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	ps := NewPlanStore(db)
	now := time.Now()
	inA := &model.OperationPlan{
		ID: "pa", ConfigID: "cfgA", Name: "A", Status: model.PlanEditing,
		PlanHash: "samehash", OpCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	inB := &model.OperationPlan{
		ID: "pb", ConfigID: "cfgB", Name: "B", Status: model.PlanEditing,
		PlanHash: "samehash", OpCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := ps.Create(inA); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if err := ps.Create(inB); err != nil {
		t.Fatalf("create B: %v", err)
	}
	gotA, err := ps.GetByHash("cfgA", "samehash")
	if err != nil {
		t.Fatalf("GetByHash cfgA: %v", err)
	}
	if gotA.ID != "pa" || gotA.ConfigID != "cfgA" {
		t.Fatalf("cfgA 误配为 %+v", gotA)
	}
	gotB, err := ps.GetByHash("cfgB", "samehash")
	if err != nil {
		t.Fatalf("GetByHash cfgB: %v", err)
	}
	if gotB.ID != "pb" || gotB.ConfigID != "cfgB" {
		t.Fatalf("cfgB 误配为 %+v", gotB)
	}
	// 不存在的配置即使哈希相同，也应 not found。
	if _, err := ps.GetByHash("cfgZ", "samehash"); err != model.ErrNotFound {
		t.Fatalf("不存在的配置应返回 ErrNotFound，实际 %v", err)
	}
}

func TestWearStoreReplace(t *testing.T) {
	db, _ := Open(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	ws := NewWearStore(db)
	entries := []model.WearEntry{
		{BlockIndex: 0, EraseCount: 10, WearPct: 1, Tier: "healthy"},
		{BlockIndex: 1, EraseCount: 850, WearPct: 85, Tier: "warning"},
	}
	if err := ws.ReplaceAll("p1", entries); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}
	got, err := ws.List("p1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[1].Tier != "warning" {
		t.Fatalf("list mismatch: %+v", got)
	}
	// 再次替换应清空旧数据。
	if err := ws.ReplaceAll("p1", entries[:1]); err != nil {
		t.Fatalf("ReplaceAll2: %v", err)
	}
	got, _ = ws.List("p1")
	if len(got) != 1 {
		t.Fatalf("二次替换后 len = %d", len(got))
	}
}

func TestCertStoreRevoke(t *testing.T) {
	db, _ := Open(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	cs := NewCertStore(db)
	c := &model.StrategyCertificate{
		ID: "cert1", PlanID: "p1", ConfigID: "cfg1", Status: model.CertPublished,
		Name: "policy", ConfigSnapshot: "{}", MappingSnapshot: "[]", PlanHash: "h",
		IssuedAt: time.Now(),
	}
	if err := cs.Insert(c); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := cs.Revoke("cert1", "reason"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	got, err := cs.Get("cert1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != model.CertRevoked || got.RevokeReason != "reason" {
		t.Fatalf("revoked mismatch: %+v", got)
	}
}
