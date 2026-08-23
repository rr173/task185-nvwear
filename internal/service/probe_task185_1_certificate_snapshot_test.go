package service

import (
	"context"
	"strings"
	"testing"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

func TestBug01_CertificatePreservesBothSnapshots(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/cert.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	ctx := context.Background()
	app := New(db)
	cfg, err := app.Configs.Create(ctx, ConfigCreateInput{Name: "cfg", DieCount: 1, PlanesPerDie: 1, BlocksPerPlane: 4, PagesPerBlock: 4, PageBytes: 512, ReservedBlocks: 1, MaxEraseCycles: 10, WarnThreshold: 80})
	if err != nil { t.Fatal(err) }
	if _, err = app.Configs.Freeze(ctx, cfg.ID); err != nil { t.Fatal(err) }
	ms := store.NewMappingStore(db)
	if err = ms.Insert(&model.LogicalMapping{ConfigID: cfg.ID, LPN: 7, PPN: 0, BlockIndex: 0, PageInBlock: 0, Version: 0}); err != nil { t.Fatal(err) }
	plan, err := app.Plans.Create(ctx, PlanCreateInput{ConfigID: cfg.ID, Name: "plan", Ops: []model.PlanOperation{{Type: model.OpWrite, LPN: 1, DestPPN: 0}}})
	if err != nil { t.Fatal(err) }
	if _, err = app.Plans.Simulate(ctx, plan.ID); err != nil { t.Fatal(err) }
	cert, err := app.Certs.Issue(ctx, plan.ID, "policy")
	if err != nil { t.Fatal(err) }
	if !strings.Contains(cert.ConfigSnapshot, `"name":"cfg"`) { t.Fatalf("config snapshot lost: %s", cert.ConfigSnapshot) }
	if !strings.Contains(cert.MappingSnapshot, `"lpn":7`) { t.Fatalf("mapping snapshot lost: %s", cert.MappingSnapshot) }
}
