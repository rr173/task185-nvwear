package store

import (
	"fmt"
	"path/filepath"
	"sync"
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

// TestPlanStoreCreateConcurrentConverges 直接验证 store 层并发收敛：
// 多个 goroutine 用不同 ID 插入相同 (config_id, plan_hash)，应只有一个成功，
// 其余收到 ErrConcurrentCreate（而非裸 UNIQUE 约束错误），且仅留一行记录。
func TestPlanStoreCreateConcurrentConverges(t *testing.T) {
	db, _ := Open(filepath.Join(t.TempDir(), "test.db"))
	defer db.Close()
	ps := NewPlanStore(db)

	const n = 40
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			errs[i] = ps.Create(&model.OperationPlan{
				ID: fmt.Sprintf("plan-%d", i), ConfigID: "cfg1", Name: "dup",
				Status: model.PlanEditing, PlanHash: "samehash", OpCount: 1,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
		}(i)
	}
	wg.Wait()

	wins := 0
	concurrent := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case IsConcurrentCreate(err):
			concurrent++
		default:
			t.Fatalf("意外错误（应为 nil 或 ErrConcurrentCreate）: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("应有且仅有一个请求成功，实际 %d", wins)
	}
	if concurrent != n-1 {
		t.Fatalf("其余 %d 应为 ErrConcurrentCreate，实际 %d", n-1, concurrent)
	}
	rows, err := ps.List("cfg1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("数据库应仅 1 行，实际 %d", len(rows))
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
