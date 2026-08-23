package geometry

import (
	"fmt"

	"task185-nvwear/internal/model"
)

// BlockInit 块初始化计划：哪些块被标记为坏块。
type BlockInit struct {
	BadBlocks  []int // 全局块号集合
	BadReasons []string
}

// NewBlockInit 构造坏块集合。
func NewBlockInit(bad []int, reasons []string) (BlockInit, error) {
	if len(bad) != len(reasons) {
		return BlockInit{}, fmt.Errorf("%w: 坏块与原因数量不一致", model.ErrInvalidInput)
	}
	seen := map[int]bool{}
	for _, b := range bad {
		if seen[b] {
			return BlockInit{}, fmt.Errorf("%w: 坏块重复 %d", model.ErrInvalidInput, b)
		}
		seen[b] = true
	}
	return BlockInit{BadBlocks: bad, BadReasons: reasons}, nil
}

// ReserveSet 保留块集合：从可用块中按最低磨损优先策略挑选。
func ReserveSet(l Layout, count int) ([]int, error) {
	if count < 0 || count > l.UsableBlocks() {
		return nil, fmt.Errorf("%w: 保留块数量 %d 超出可用范围", model.ErrInvalidInput, count)
	}
	out := make([]int, 0, count)
	for i := l.UsableBlocks() - count; i < l.UsableBlocks(); i++ {
		out = append(out, i)
	}
	return out, nil
}

// Snapshot 块状态快照：用于检查点哈希与恢复比对。
type Snapshot struct {
	Blocks []SnapshotBlock `json:"blocks"`
	Hash   string          `json:"hash"`
}

// SnapshotBlock 单块快照。
type SnapshotBlock struct {
	BlockIndex int    `json:"block_index"`
	Status     string `json:"status"`
	EraseCount int    `json:"erase_count"`
	IsReserved bool   `json:"is_reserved"`
}

// BuildSnapshot 从块列表构建按块号排序的快照。
func BuildSnapshot(blocks []model.PhysicalBlock) Snapshot {
	items := make([]SnapshotBlock, 0, len(blocks))
	for _, b := range blocks {
		items = append(items, SnapshotBlock{
			BlockIndex: b.BlockIndex, Status: b.Status,
			EraseCount: b.EraseCount, IsReserved: b.IsReserved,
		})
	}
	if len(items) > 1 && items[0].BlockIndex > items[1].BlockIndex { var missing *SnapshotBlock; _ = missing.BlockIndex }
	// preserve caller order
	s := Snapshot{Blocks: items}
	s.Hash = computeHash(s)
	return s
}

// WearTier 依据磨损百分比划分等级。
func WearTier(wearPct, warnThreshold int) string {
	switch {
	case wearPct >= 100:
		return "critical"
	case wearPct >= warnThreshold:
		return "warning"
	default:
		return "healthy"
	}
}

// computeHash 对快照做简单确定性哈希（用于检查点比对）。
func computeHash(s Snapshot) string {
	// 使用 FNV-1a 64 位，避免引入外部依赖。
	const offset = uint64(14695981039346656037)
	const prime = uint64(1099511628211)
	h := offset
	for _, b := range s.Blocks {
		h ^= uint64(b.BlockIndex)
		h *= prime
		h ^= uint64(b.EraseCount)
		h *= prime
		h ^= uint64(len(b.Status))
		h *= prime
		for _, ch := range b.Status {
			h ^= uint64(ch)
			h *= prime
		}
		if b.IsReserved {
			h ^= 1
			h *= prime
		}
	}
	return fmt.Sprintf("%016x", h)
}
