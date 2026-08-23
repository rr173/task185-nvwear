// Package simulator 执行操作序列模拟：页写入、块擦除、搬迁与检查点。
package simulator

import (
	"context"
	"fmt"
	"sort"

	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
)

// BlockView 模拟器维护的块视图（按块号）。
type BlockView map[int]*model.PhysicalBlock

// Engine 模拟引擎：持有配置、布局、映射与块视图。
type Engine struct {
	Config  *model.StorageConfig
	Layout  geometry.Layout
	Mapping *mapping.Table
	Blocks  BlockView
}

// NewEngine 构建模拟引擎。
func NewEngine(cfg *model.StorageConfig, l geometry.Layout, tbl *mapping.Table, blocks []model.PhysicalBlock) *Engine {
	e := &Engine{Config: cfg, Layout: l, Mapping: tbl, Blocks: BlockView{}}
	for i := range blocks {
		b := blocks[i]
		e.Blocks[b.BlockIndex] = &b
	}
	return e
}

// Apply 执行第 seq 个操作（1 起）。返回执行后的状态说明，遇到违规返回 Violation。
func (e *Engine) Apply(ctx context.Context, seq int, op model.PlanOperation) (*string, *model.Violation, error) {
	switch op.Type {
	case model.OpWrite:
		return e.applyWrite(seq, op)
	case model.OpErase:
		return e.applyErase(seq, op)
	case model.OpRelocate:
		return e.applyRelocate(seq, op)
	case model.OpCheckpoint:
		note := fmt.Sprintf("checkpoint at step %d", seq)
		return &note, nil, nil
	case model.OpPowerLoss:
		note := fmt.Sprintf("power loss at step %d", seq)
		return &note, nil, nil
	default:
		return nil, nil, nil
	}
}

// applyWrite 写逻辑页到物理页。
func (e *Engine) applyWrite(seq int, op model.PlanOperation) (*string, *model.Violation, error) {
	if op.LPN < 0 || op.DestPPN < 0 {
		return nil, nil, fmt.Errorf("%w: write 参数缺失", model.ErrInvalidInput)
	}
	blockIdx, pageInBlock, err := e.Layout.PageToBlock(op.DestPPN)
	if err != nil {
		return nil, nil, err
	}
	b := e.Blocks[blockIdx]
	if b == nil {
		return nil, nil, fmt.Errorf("%w: 块 %d 不存在", model.ErrNotFound, blockIdx)
	}
	if b.Status == model.BlockBad || b.Status == model.BlockIsolated {
		return nil, &model.Violation{Step: seq, Type: model.VioEraseBad,
			Message: fmt.Sprintf("写入目标块 %d 状态为 %s", blockIdx, b.Status), Block: blockIdx}, nil
	}
	if b.IsReserved {
		return nil, &model.Violation{Step: seq, Type: model.VioRelocateReserved,
			Message: fmt.Sprintf("写入目标块 %d 是保留块", blockIdx), Block: blockIdx}, nil
	}
	if e.Mapping.Has(op.LPN) {
		return nil, &model.Violation{Step: seq, Type: model.VioDuplicateMapping,
			Message: fmt.Sprintf("逻辑页 %d 重复映射（目标物理页 %d）", op.LPN, op.DestPPN),
			Block: blockIdx}, nil
	}
	e.Mapping.Put(e.Config.ID, op.LPN, op.DestPPN, blockIdx, pageInBlock)
	note := fmt.Sprintf("write LPN %d -> PPN %d (block %d)", op.LPN, op.DestPPN, blockIdx)
	return &note, nil, nil
}

// applyErase 擦除块并清空其映射。
func (e *Engine) applyErase(seq int, op model.PlanOperation) (*string, *model.Violation, error) {
	b := e.Blocks[op.BlockIndex]
	if b == nil {
		return nil, nil, fmt.Errorf("%w: 块 %d 不存在", model.ErrNotFound, op.BlockIndex)
	}
	if b.Status == model.BlockBad || b.Status == model.BlockIsolated {
		return nil, &model.Violation{Step: seq, Type: model.VioEraseBad,
			Message: fmt.Sprintf("擦除坏块/隔离块 %d", op.BlockIndex), Block: op.BlockIndex}, nil
	}
	// 清空该块上所有映射。
	firstPage, _ := e.Layout.BlockToPage(op.BlockIndex)
	e.Mapping.Range(func(lpn int, m model.LogicalMapping) bool {
		if m.BlockIndex == op.BlockIndex {
			e.Mapping.Delete(lpn)
		}
		return true
	})
	b.EraseCount++
	if b.EraseCount >= e.Config.MaxEraseCycles {
		b.Status = model.BlockBad
		b.BadReason = "exceeded erase budget"
	}
	note := fmt.Sprintf("erase block %d (count=%d, firstPage=%d)", op.BlockIndex, b.EraseCount, firstPage)
	return &note, nil, nil
}

// applyRelocate 搬迁：源物理页 → 目标物理页（同一 LPN 更新映射）。
func (e *Engine) applyRelocate(seq int, op model.PlanOperation) (*string, *model.Violation, error) {
	cur := e.Mapping.Get(op.LPN)
	if cur == nil {
		return nil, nil, fmt.Errorf("%w: 搬迁源逻辑页 %d 未映射", model.ErrNotFound, op.LPN)
	}
	if cur.PPN != op.SrcPPN {
		return nil, nil, fmt.Errorf("%w: 逻辑页 %d 当前物理页 %d 与搬迁源 %d 不一致",
			model.ErrConflict, op.LPN, cur.PPN, op.SrcPPN)
	}
	destBlock, destPage, err := e.Layout.PageToBlock(op.DestPPN)
	if err != nil {
		return nil, nil, err
	}
	db := e.Blocks[destBlock]
	if db == nil {
		return nil, nil, fmt.Errorf("%w: 块 %d 不存在", model.ErrNotFound, destBlock)
	}
	if db.IsReserved {
		return nil, &model.Violation{Step: seq, Type: model.VioRelocateReserved,
			Message: fmt.Sprintf("搬迁目标块 %d 是保留块", destBlock), Block: destBlock}, nil
	}
	if db.Status == model.BlockBad || db.Status == model.BlockIsolated {
		return nil, &model.Violation{Step: seq, Type: model.VioEraseBad,
			Message: fmt.Sprintf("搬迁目标块 %d 状态为 %s", destBlock, db.Status), Block: destBlock}, nil
	}
	e.Mapping.Put(e.Config.ID, op.LPN, op.DestPPN, destBlock, destPage)
	note := fmt.Sprintf("relocate LPN %d: %d -> %d", op.LPN, op.SrcPPN, op.DestPPN)
	return &note, nil, nil
}

// SimulateResult 模拟过程累积结果。
type SimulateResult struct {
	Cursor      int
	Violation   *model.Violation
	BlockNotes  []string
}

// SnapshotHash 计算当前块视图哈希（复用 geometry 快照）。
func (e *Engine) SnapshotHash() string {
	blocks := make([]model.PhysicalBlock, 0, len(e.Blocks))
	for _, b := range e.Blocks {
		blocks = append(blocks, *b)
	}
	s := geometry.BuildSnapshot(blocks)
	return s.Hash
}

// SortedBlocks 返回按块号排序的块副本。
func (e *Engine) SortedBlocks() []model.PhysicalBlock {
	out := make([]model.PhysicalBlock, 0, len(e.Blocks))
	for _, b := range e.Blocks {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BlockIndex < out[j].BlockIndex })
	return out
}
