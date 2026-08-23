package service

import (
	"context"
	"testing"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

func TestBug02_RecoveryReplaysFromLatestCheckpoint(t *testing.T) {
	db, err := store.Open(t.TempDir()+"/recovery.db"); if err != nil { t.Fatal(err) }; defer db.Close()
	ctx := context.Background(); app := New(db)
	cfg, err := app.Configs.Create(ctx, ConfigCreateInput{Name:"cfg", DieCount:1, PlanesPerDie:1, BlocksPerPlane:8, PagesPerBlock:4, PageBytes:512, ReservedBlocks:1, MaxEraseCycles:10, WarnThreshold:80}); if err != nil { t.Fatal(err) }
	if _, err = app.Configs.Freeze(ctx, cfg.ID); err != nil { t.Fatal(err) }
	ops := []model.PlanOperation{{Type:model.OpWrite,LPN:1,DestPPN:0},{Type:model.OpCheckpoint},{Type:model.OpWrite,LPN:2,DestPPN:1},{Type:model.OpCheckpoint},{Type:model.OpWrite,LPN:3,DestPPN:2},{Type:model.OpPowerLoss}}
	p, err := app.Plans.Create(ctx, PlanCreateInput{ConfigID:cfg.ID,Name:"recovery",Ops:ops}); if err != nil { t.Fatal(err) }
	first, err := app.Plans.Checkpoint(ctx,p.ID,2); if err != nil { t.Fatal(err) }
	second, err := app.Plans.Checkpoint(ctx,p.ID,4); if err != nil { t.Fatal(err) }
	rep, err := app.Plans.Recover(ctx,p.ID); if err != nil { t.Fatal(err) }
	if !rep.Recoverable || !rep.IntegrityOK { t.Fatalf("recovery failed: %+v", rep) }
	if rep.FromCheckpoint != second.ID || rep.FromCheckpoint == first.ID { t.Fatalf("wrong checkpoint: %+v", rep) }
	if rep.Replayed != 1 || rep.ReplaySteps != 2 { t.Fatalf("wrong replay count: %+v", rep) }
}
