package service

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

// setupTestApp 构造一个带冻结配置与初始映射的 App，供计划测试使用。
func setupTestApp(t *testing.T) *App {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := New(db)

	ctx := context.Background()
	cfg, err := app.Configs.Create(ctx, ConfigCreateInput{
		Name: "test-ssd", DieCount: 2, PlanesPerDie: 2, BlocksPerPlane: 16,
		PagesPerBlock: 32, PageBytes: 4096, ReservedBlocks: 2,
		MaxEraseCycles: 1000, WarnThreshold: 80,
	})
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if _, err := app.Configs.Freeze(ctx, cfg.ID); err != nil {
		t.Fatalf("freeze config: %v", err)
	}
	if _, err := app.Configs.InitMapping(ctx, cfg.ID); err != nil {
		t.Fatalf("init mapping: %v", err)
	}
	return app
}

// TestPlanCreateConcurrentConverges 并发同配置同序列创建必须收敛到同一已持久化计划：
// 无数据库错误、无孤立/重复行、仅一套操作。
func TestPlanCreateConcurrentConverges(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ops := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 1, DestPPN: 1*32 + 0},
		{Type: model.OpWrite, LPN: 2, DestPPN: 2*32 + 0},
		{Type: model.OpErase, BlockIndex: 3},
		{Type: model.OpCheckpoint, Note: "cp"},
	}
	// 取出冻结配置 ID。
	cfgs, _ := app.Configs.List(ctx)
	configID := cfgs[0].ID

	const n = 50
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []*model.OperationPlan
		errs    []error
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			p, err := app.Plans.Create(ctx, PlanCreateInput{
				ConfigID: configID, Name: "concurrent-plan", Ops: ops,
			})
			mu.Lock()
			results = append(results, p)
			errs = append(errs, err)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	// 1) 全部成功，无错误。
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d 返回错误: %v", i, err)
		}
	}
	// 2) 全部收敛到同一 planID。
	first := results[0]
	if first == nil {
		t.Fatal("首个结果为空")
	}
	for i, p := range results {
		if p == nil {
			t.Fatalf("goroutine %d 结果为空", i)
		}
		if p.ID != first.ID {
			t.Fatalf("goroutine %d 收敛失败: planID=%s, 期望=%s", i, p.ID, first.ID)
		}
		if p.PlanHash != first.PlanHash {
			t.Fatalf("goroutine %d 哈希不一致", i)
		}
	}

	// 3) 数据库中该哈希仅 1 行计划、1 套操作。
	plans, err := app.Plans.List(ctx, configID)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	sameHash := 0
	for _, p := range plans {
		if p.PlanHash == first.PlanHash {
			sameHash++
		}
	}
	if sameHash != 1 {
		t.Fatalf("同哈希计划应仅 1 行，实际 %d", sameHash)
	}
	gotOps, err := app.Plans.Ops(ctx, first.ID)
	if err != nil {
		t.Fatalf("list ops: %v", err)
	}
	if len(gotOps) != len(ops) {
		t.Fatalf("操作数应为 %d，实际 %d", len(ops), len(gotOps))
	}
	// 4) 二次（非并发）创建同样收敛到既有计划。
	again, err := app.Plans.Create(ctx, PlanCreateInput{
		ConfigID: configID, Name: "concurrent-plan-again", Ops: ops,
	})
	if err != nil {
		t.Fatalf("二次创建: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("二次创建应收敛，%s != %s", again.ID, first.ID)
	}
}
