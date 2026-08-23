// Package budget 评估磨损预算：热块均衡、保留块下限与磨损曲线。
package budget

import (
	"fmt"
	"sort"

	"task185-nvwear/internal/model"
)

// Evaluator 磨损预算评估器。
type Evaluator struct {
	MaxEraseCycles int
	WarnThreshold  int
	ReservedFloor  int // 保留块数量下限
	UsableBlocks   int
	HotBlockRatio  int // 热块判定：单块磨损百分比（0-100）
}

// New 构建评估器。
func New(maxErase, warnThreshold, reservedFloor, usableBlocks, hotBlockPct int) (*Evaluator, error) {
	if maxErase <= 0 || warnThreshold <= 0 || warnThreshold > 100 || reservedFloor < 0 {
		return nil, fmt.Errorf("%w: 预算参数非法", model.ErrInvalidInput)
	}
	if hotBlockPct <= 0 || hotBlockPct > 100 {
		return nil, fmt.Errorf("%w: 热块阈值非法", model.ErrInvalidInput)
	}
	return &Evaluator{
		MaxEraseCycles: maxErase, WarnThreshold: warnThreshold,
		ReservedFloor: reservedFloor, UsableBlocks: usableBlocks, HotBlockRatio: hotBlockPct,
	}, nil
}

// WearEntry 由块计算磨损点。
func (e *Evaluator) WearEntry(b model.PhysicalBlock) model.WearEntry {
	pct := 0
	if e.MaxEraseCycles > 0 {
		pct = b.EraseCount * 100 / e.MaxEraseCycles
		if pct > 100 {
			pct = 100
		}
	}
	tier := "healthy"
	switch {
	case pct >= 100:
		tier = "critical"
	case pct > e.WarnThreshold:
		tier = "warning"
	}
	return model.WearEntry{
		BlockIndex: b.BlockIndex, EraseCount: b.EraseCount,
		WearPct: pct, Tier: tier, IsReserved: b.IsReserved,
	}
}

// Check 校验预算：返回第一个违反（如有）。
func (e *Evaluator) Check(step int, blocks []model.PhysicalBlock) *model.Violation {
	// 1. 热块检查：任何非保留块磨损超过阈值。
	for _, b := range blocks {
		if b.IsReserved || b.Status == model.BlockBad || b.Status == model.BlockIsolated {
			continue
		}
		pct := b.EraseCount * 100 / e.MaxEraseCycles
		if pct > e.HotBlockRatio {
			return &model.Violation{Step: step, Type: model.VioHotBlock,
				Message: fmt.Sprintf("块 %d 磨损 %d%% 超过热块阈值 %d%%",
					b.BlockIndex, pct, e.HotBlockRatio), Block: b.BlockIndex}
		}
	}
	// 2. 保留块下限检查。
	reserved := 0
	for _, b := range blocks {
		if b.IsReserved {
			reserved++
		}
	}
	if reserved < e.ReservedFloor {
		return &model.Violation{Step: step, Type: model.VioReserveBelowFloor,
			Message: fmt.Sprintf("保留块 %d 低于下限 %d", reserved, e.ReservedFloor)}
	}
	return nil
}

// Balance 计算磨损均衡度：max/min 磨损百分比（保留与坏块除外）。
func (e *Evaluator) Balance(blocks []model.PhysicalBlock) (maxPct, minPct, avgPct int, ok bool) {
	var vals []int
	for _, b := range blocks {
		if b.IsReserved || b.Status == model.BlockBad || b.Status == model.BlockIsolated {
			continue
		}
		pct := b.EraseCount * 100 / e.MaxEraseCycles
		vals = append(vals, pct)
	}
	if len(vals) == 0 {
		return 0, 0, 0, false
	}
	sort.Ints(vals)
	maxPct, minPct = vals[len(vals)-1], vals[0]
	sum := 0
	for _, v := range vals {
		sum += v
	}
	avgPct = sum / len(vals)
	ok = true
	return
}
