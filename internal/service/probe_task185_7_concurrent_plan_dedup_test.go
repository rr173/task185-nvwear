package service

import (
	"context"
	"sync"
	"testing"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

func TestBug07_ConcurrentIdenticalPlansConverge(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/plans.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	ctx := context.Background()
	app := New(db)
	cfg, err := app.Configs.Create(ctx, ConfigCreateInput{Name: "cfg", DieCount: 1, PlanesPerDie: 1, BlocksPerPlane: 8, PagesPerBlock: 4, PageBytes: 512, ReservedBlocks: 1, MaxEraseCycles: 10, WarnThreshold: 80})
	if err != nil { t.Fatal(err) }
	if _, err = app.Configs.Freeze(ctx, cfg.ID); err != nil { t.Fatal(err) }
	in := PlanCreateInput{ConfigID: cfg.ID, Name: "same", Ops: []model.PlanOperation{{Type: model.OpWrite, LPN: 1, DestPPN: 0}}}
	ids := make(chan string, 20)
	errs := make(chan error, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p, e := app.Plans.Create(ctx, in); if e != nil { errs <- e; return }; ids <- p.ID }()
	}
	wg.Wait(); close(ids); close(errs)
	if len(errs) != 0 { t.Fatalf("concurrent create errors: %d", len(errs)) }
	seen := map[string]bool{}
	for id := range ids { seen[id] = true }
	if len(seen) != 1 { t.Fatalf("expected one plan, got %d ids", len(seen)) }
	plans, err := app.Plans.List(ctx, cfg.ID)
	if err != nil { t.Fatal(err) }
	if len(plans) != 1 { t.Fatalf("expected one persisted plan, got %d", len(plans)) }
}
