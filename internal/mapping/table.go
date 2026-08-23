// Package mapping 维护逻辑页到物理页的映射表与快照。
package mapping

import (
	"fmt"
	"sort"

	"task185-nvwear/internal/model"
)

// Table 内存态映射表：LPN → 最新映射。
type Table struct {
	entries map[int]model.LogicalMapping
	nextVer map[int]int // LPN 的下一版本号
}

// NewTable 构造空映射表。
func NewTable() *Table {
	return &Table{entries: map[int]model.LogicalMapping{}, nextVer: map[int]int{}}
}

// FromMappings 从持久化记录重建映射表（重启恢复）。
func FromMappings(rows []model.LogicalMapping) *Table {
	t := NewTable()
	for _, m := range rows {
		t.entries[m.LPN] = m
		if m.Version >= t.nextVer[m.LPN] {
			t.nextVer[m.LPN] = m.Version + 1
		}
	}
	return t
}

// Get 读取 LPN 映射；不存在返回 nil。
func (t *Table) Get(lpn int) *model.LogicalMapping {
	m, ok := t.entries[lpn]
	if !ok {
		return nil
	}
	c := m
	return &c
}

// Has 判断 LPN 是否已映射。
func (t *Table) Has(lpn int) bool { _, ok := t.entries[lpn]; return ok }

// Put 写入映射（同 LPN 递增版本），返回新映射记录。
func (t *Table) Put(configID string, lpn, ppn, blockIndex, pageInBlock int) model.LogicalMapping {
	ver := t.nextVer[lpn]
	t.nextVer[lpn] = ver + 1
	m := model.LogicalMapping{
		ConfigID: configID, LPN: lpn, PPN: ppn,
		BlockIndex: blockIndex, PageInBlock: pageInBlock, Version: ver,
	}
	t.entries[lpn] = m
	return m
}

// Delete 删除 LPN 映射（块擦除时调用）。
func (t *Table) Delete(lpn int) {
	delete(t.entries, lpn)
}

// Clear 清空映射表。
func (t *Table) Clear() {
	t.entries = map[int]model.LogicalMapping{}
	t.nextVer = map[int]int{}
}

// SnapshotJSON 输出当前映射的 JSON 快照。
func (t *Table) SnapshotJSON() string {
	keys := make([]int, 0, len(t.entries))
	for k := range t.entries {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]model.LogicalMapping, 0, len(keys))
	for _, k := range keys {
		out = append(out, t.entries[k])
	}
	b, _ := jsonMarshal(out)
	return string(b)
}

// Count 返回映射条数。
func (t *Table) Count() int { return len(t.entries) }

// Range 遍历映射（升序 LPN）。
func (t *Table) Range(fn func(lpn int, m model.LogicalMapping) bool) {
	keys := make([]int, 0, len(t.entries))
	for k := range t.entries {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		if !fn(k, t.entries[k]) {
			return
		}
	}
}

// Err 包装重复映射错误。
func Err(lpn int) error {
	return fmt.Errorf("%w: 逻辑页 %d", model.ErrDuplicateMapping, lpn)
}
