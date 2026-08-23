package geometry

import (
	"testing"

	"task185-nvwear/internal/model"
)

func TestNewLayout(t *testing.T) {
	l, err := NewLayout(2, 2, 16, 32, 4096, 4)
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	if l.TotalBlocks() != 2*2*16 {
		t.Fatalf("TotalBlocks = %d, want %d", l.TotalBlocks(), 2*2*16)
	}
	if l.UsableBlocks() != 2*2*16-4 {
		t.Fatalf("UsableBlocks = %d, want %d", l.UsableBlocks(), 2*2*16-4)
	}
	if l.TotalPages() != 2*2*16*32 {
		t.Fatalf("TotalPages = %d", l.TotalPages())
	}
}

func TestNewLayoutInvalid(t *testing.T) {
	if _, err := NewLayout(0, 2, 16, 32, 4096, 4); err == nil {
		t.Fatal("die=0 应报错")
	}
	if _, err := NewLayout(2, 2, 16, 32, 4096, 100); err == nil {
		t.Fatal("保留块 >= 总块应报错")
	}
}

func TestLayoutIndexLocationRoundTrip(t *testing.T) {
	l, _ := NewLayout(4, 2, 32, 64, 4096, 4)
	for idx := 0; idx < l.TotalBlocks(); idx++ {
		die, plane, blk, err := l.IndexToLocation(idx)
		if err != nil {
			t.Fatalf("IndexToLocation(%d): %v", idx, err)
		}
		back, err := l.LocationToIndex(die, plane, blk)
		if err != nil {
			t.Fatalf("LocationToIndex(%d,%d,%d): %v", die, plane, blk, err)
		}
		if back != idx {
			t.Fatalf("round trip %d -> %d", idx, back)
		}
	}
}

func TestPageToBlock(t *testing.T) {
	l, _ := NewLayout(2, 2, 16, 32, 4096, 4)
	blk, page, err := l.PageToBlock(33)
	if err != nil {
		t.Fatalf("PageToBlock: %v", err)
	}
	if blk != 1 || page != 1 {
		t.Fatalf("ppn=33 -> block=%d page=%d, want 1/1", blk, page)
	}
	if _, _, err := l.PageToBlock(l.TotalPages()); err == nil {
		t.Fatal("越界 ppn 应报错")
	}
}

func TestComputeGeometry(t *testing.T) {
	c := &model.StorageConfig{
		Name: "t", DieCount: 2, PlanesPerDie: 2, BlocksPerPlane: 16,
		PagesPerBlock: 32, PageBytes: 4096, ReservedBlocks: 4,
		MaxEraseCycles: 1000, WarnThreshold: 80,
	}
	if err := ComputeGeometry(c); err != nil {
		t.Fatalf("ComputeGeometry: %v", err)
	}
	if c.TotalBytes != int64(2*2*16*32*4096) {
		t.Fatalf("TotalBytes = %d", c.TotalBytes)
	}
	c.MaxEraseCycles = 0
	if err := ComputeGeometry(c); err == nil {
		t.Fatal("磨损预算 0 应报错")
	}
}

func TestReserveSet(t *testing.T) {
	l, _ := NewLayout(2, 2, 16, 32, 4096, 4)
	rs, err := ReserveSet(l, 4)
	if err != nil {
		t.Fatalf("ReserveSet: %v", err)
	}
	if len(rs) != 4 {
		t.Fatalf("reserve len = %d", len(rs))
	}
	if rs[0] != l.UsableBlocks()-4 {
		t.Fatalf("reserve 应取末尾块，got %v", rs)
	}
}

func TestWearTier(t *testing.T) {
	if got := WearTier(100, 80); got != "critical" {
		t.Fatalf("100%% -> %s", got)
	}
	if got := WearTier(85, 80); got != "warning" {
		t.Fatalf("85%% -> %s", got)
	}
	if got := WearTier(10, 80); got != "healthy" {
		t.Fatalf("10%% -> %s", got)
	}
}

func TestBuildSnapshotDeterministic(t *testing.T) {
	blocks := []model.PhysicalBlock{
		{BlockIndex: 1, Status: model.BlockAvailable, EraseCount: 5},
		{BlockIndex: 0, Status: model.BlockBad, EraseCount: 0},
	}
	a := BuildSnapshot(blocks)
	b := BuildSnapshot(blocks)
	if a.Hash != b.Hash {
		t.Fatalf("快照哈希应确定: %s vs %s", a.Hash, b.Hash)
	}
	if len(a.Blocks) != 2 || a.Blocks[0].BlockIndex != 0 {
		t.Fatalf("快照应按块号排序: %+v", a.Blocks)
	}
}
