package simulator

import (
	"context"
	"testing"

	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := &model.StorageConfig{
		ID: "cfg", DieCount: 2, PlanesPerDie: 2, BlocksPerPlane: 16,
		PagesPerBlock: 32, PageBytes: 4096, ReservedBlocks: 2,
		MaxEraseCycles: 100, WarnThreshold: 80,
	}
	l, err := geometry.NewLayout(cfg.DieCount, cfg.PlanesPerDie, cfg.BlocksPerPlane,
		cfg.PagesPerBlock, cfg.PageBytes, cfg.ReservedBlocks)
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	cfg.TotalBlocks = l.TotalBlocks()
	blocks := make([]model.PhysicalBlock, 0, l.TotalBlocks())
	for i := 0; i < l.TotalBlocks(); i++ {
		b := model.PhysicalBlock{ConfigID: cfg.ID, BlockIndex: i, Status: model.BlockAvailable}
		if i == 0 {
			b.Status = model.BlockBad
		}
		if i >= l.UsableBlocks() {
			b.IsReserved = true
			b.Status = model.BlockReserved
		}
		blocks = append(blocks, b)
	}
	return NewEngine(cfg, l, mapping.NewTable(), blocks)
}

func TestApplyWrite(t *testing.T) {
	e := testEngine(t)
	note, vio, err := e.Apply(context.Background(), 1, model.PlanOperation{
		Type: model.OpWrite, LPN: 1, DestPPN: 1*32 + 0,
	})
	if err != nil || vio != nil {
		t.Fatalf("write: err=%v vio=%v", err, vio)
	}
	if note == nil {
		t.Fatal("note 为空")
	}
	if !e.Mapping.Has(1) {
		t.Fatal("LPN1 应已映射")
	}
}

func TestApplyWriteDuplicate(t *testing.T) {
	e := testEngine(t)
	_, _, _ = e.Apply(context.Background(), 1, model.PlanOperation{
		Type: model.OpWrite, LPN: 1, DestPPN: 32,
	})
	_, vio, err := e.Apply(context.Background(), 2, model.PlanOperation{
		Type: model.OpWrite, LPN: 1, DestPPN: 64,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if vio == nil || vio.Type != model.VioDuplicateMapping {
		t.Fatalf("应产生重复映射违规，got %+v", vio)
	}
}

func TestApplyWriteToReserved(t *testing.T) {
	e := testEngine(t)
	l, _ := geometry.NewLayout(2, 2, 16, 32, 4096, 2)
	_, vio, err := e.Apply(context.Background(), 1, model.PlanOperation{
		Type: model.OpWrite, LPN: 9, DestPPN: l.UsableBlocks()*32,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if vio == nil || vio.Type != model.VioRelocateReserved {
		t.Fatalf("应产生保留块违规，got %+v", vio)
	}
}

func TestApplyEraseBadBlock(t *testing.T) {
	e := testEngine(t)
	_, vio, err := e.Apply(context.Background(), 1, model.PlanOperation{
		Type: model.OpErase, BlockIndex: 0,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if vio == nil || vio.Type != model.VioEraseBad {
		t.Fatalf("应产生擦除坏块违规，got %+v", vio)
	}
}

func TestApplyEraseClearsMapping(t *testing.T) {
	e := testEngine(t)
	_, _, _ = e.Apply(context.Background(), 1, model.PlanOperation{
		Type: model.OpWrite, LPN: 1, DestPPN: 32,
	})
	_, _, err := e.Apply(context.Background(), 2, model.PlanOperation{
		Type: model.OpErase, BlockIndex: 1,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if e.Mapping.Has(1) {
		t.Fatal("擦除后 LPN1 映射应被清除")
	}
	if e.Blocks[1].EraseCount != 1 {
		t.Fatalf("擦除计数 = %d", e.Blocks[1].EraseCount)
	}
}

func TestRunnerRunStopsAtViolation(t *testing.T) {
	e := testEngine(t)
	r := NewRunner(e)
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite, LPN: 1, DestPPN: 32},
		{Seq: 2, Type: model.OpWrite, LPN: 1, DestPPN: 64}, // 重复映射
		{Seq: 3, Type: model.OpWrite, LPN: 2, DestPPN: 96},
	}
	if err := r.Run(context.Background(), ops, 0); err != nil {
		t.Fatalf("run: %v", err)
	}
	if r.FirstVio == nil || r.FirstVio.Step != 2 || r.Cursor() != 1 {
		t.Fatalf("应在第 2 步停止: vio=%+v cursor=%d", r.FirstVio, r.Cursor())
	}
}

func TestRunnerRunResume(t *testing.T) {
	e := testEngine(t)
	r := NewRunner(e)
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite, LPN: 1, DestPPN: 32},
		{Seq: 2, Type: model.OpWrite, LPN: 2, DestPPN: 64},
	}
	if err := r.Run(context.Background(), ops, 1); err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if r.Cursor() != 1 {
		t.Fatalf("续跑后 cursor = %d", r.Cursor())
	}
}
