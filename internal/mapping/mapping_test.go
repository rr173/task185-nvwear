package mapping

import (
	"testing"

	"task185-nvwear/internal/model"
)

func TestTablePutGetVersion(t *testing.T) {
	tbl := NewTable()
	tbl.Put("cfg", 1, 100, 3, 4)
	tbl.Put("cfg", 1, 200, 6, 8)
	m := tbl.Get(1)
	if m == nil {
		t.Fatal("Get(1) = nil")
	}
	if m.PPN != 200 || m.Version != 1 {
		t.Fatalf("最新映射应为 PPN=200 ver=1, got %+v", m)
	}
	if !tbl.Has(1) {
		t.Fatal("Has(1) = false")
	}
	if tbl.Has(2) {
		t.Fatal("Has(2) = true")
	}
}

func TestTableDelete(t *testing.T) {
	tbl := NewTable()
	tbl.Put("cfg", 1, 100, 3, 4)
	tbl.Delete(1)
	if tbl.Has(1) {
		t.Fatal("删除后仍存在")
	}
	if tbl.Count() != 0 {
		t.Fatalf("Count = %d", tbl.Count())
	}
}

func TestFromMappings(t *testing.T) {
	rows := []model.LogicalMapping{
		{ConfigID: "cfg", LPN: 1, PPN: 100, Version: 0},
		{ConfigID: "cfg", LPN: 1, PPN: 200, Version: 1},
		{ConfigID: "cfg", LPN: 2, PPN: 300, Version: 0},
	}
	tbl := FromMappings(rows)
	if m := tbl.Get(1); m == nil || m.PPN != 200 || m.Version != 1 {
		t.Fatalf("重建后 LPN1 = %+v", m)
	}
	if tbl.Count() != 2 {
		t.Fatalf("Count = %d", tbl.Count())
	}
}

func TestSnapshotJSON(t *testing.T) {
	tbl := NewTable()
	tbl.Put("cfg", 2, 200, 6, 8)
	tbl.Put("cfg", 1, 100, 3, 4)
	s := tbl.SnapshotJSON()
	if s == "" {
		t.Fatal("快照为空")
	}
	if tbl.Count() != 2 {
		t.Fatalf("Count = %d", tbl.Count())
	}
}
