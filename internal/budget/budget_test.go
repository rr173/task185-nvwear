package budget

import (
	"testing"

	"task185-nvwear/internal/model"
)

func TestEvaluatorHotBlock(t *testing.T) {
	e, err := New(1000, 80, 2, 100, 80)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	blocks := []model.PhysicalBlock{
		{BlockIndex: 0, Status: model.BlockAvailable, EraseCount: 850},
	}
	vio := e.Check(10, blocks)
	if vio == nil || vio.Type != model.VioHotBlock {
		t.Fatalf("应产生热块违规: %+v", vio)
	}
}

func TestEvaluatorReserveFloor(t *testing.T) {
	e, _ := New(1000, 80, 3, 100, 80)
	blocks := []model.PhysicalBlock{
		{BlockIndex: 0, Status: model.BlockAvailable, EraseCount: 10},
		{BlockIndex: 1, IsReserved: true, Status: model.BlockReserved, EraseCount: 0},
	}
	vio := e.Check(5, blocks)
	if vio == nil || vio.Type != model.VioReserveBelowFloor {
		t.Fatalf("应产生保留块下限违规: %+v", vio)
	}
}

func TestEvaluatorPass(t *testing.T) {
	e, _ := New(1000, 80, 2, 100, 80)
	blocks := []model.PhysicalBlock{
		{BlockIndex: 0, Status: model.BlockAvailable, EraseCount: 10},
		{BlockIndex: 1, IsReserved: true, Status: model.BlockReserved, EraseCount: 0},
		{BlockIndex: 2, IsReserved: true, Status: model.BlockReserved, EraseCount: 0},
	}
	if vio := e.Check(1, blocks); vio != nil {
		t.Fatalf("不应违规: %+v", vio)
	}
}

func TestBalance(t *testing.T) {
	e, _ := New(1000, 80, 0, 100, 80)
	blocks := []model.PhysicalBlock{
		{BlockIndex: 0, EraseCount: 10},
		{BlockIndex: 1, EraseCount: 500},
	}
	maxPct, minPct, _, ok := e.Balance(blocks)
	if !ok || maxPct != 50 || minPct != 1 {
		t.Fatalf("balance = %d/%d ok=%v", maxPct, minPct, ok)
	}
}

func TestWearEntry(t *testing.T) {
	e, _ := New(1000, 80, 0, 100, 80)
	we := e.WearEntry(model.PhysicalBlock{BlockIndex: 0, EraseCount: 900})
	if we.WearPct != 90 || we.Tier != "warning" {
		t.Fatalf("wear = %+v", we)
	}
	we = e.WearEntry(model.PhysicalBlock{BlockIndex: 1, EraseCount: 1000})
	if we.Tier != "critical" {
		t.Fatalf("1000 擦除应为 critical, got %s", we.Tier)
	}
}

func TestNewInvalid(t *testing.T) {
	if _, err := New(0, 80, 0, 100, 80); err == nil {
		t.Fatal("预算 0 应报错")
	}
	if _, err := New(1000, 101, 0, 100, 80); err == nil {
		t.Fatal("阈值 >100 应报错")
	}
}
