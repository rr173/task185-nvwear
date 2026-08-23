package simulator

import (
	"context"
	"fmt"

	"task185-nvwear/internal/model"
)

// Runner 执行操作序列并记录模拟轨迹。
type Runner struct {
	Engine   *Engine
	Steps    []StepRecord
	FirstVio *model.Violation
}

// StepRecord 单步模拟记录。
type StepRecord struct {
	Seq      int
	Type     model.OpType
	Note     string
	Snapshot string // 步骤后的映射快照（JSON）
	WearHash string // 步骤后的磨损哈希
}

// NewRunner 构建执行器。
func NewRunner(e *Engine) *Runner { return &Runner{Engine: e} }

// Run 从起始游标执行到序列末尾（或首个违反点）。
// ops 按 seq 升序；start 为已执行步骤数（恢复续跑时 >0）。
func (r *Runner) Run(ctx context.Context, ops []model.PlanOperation, start int) error {
	if start > len(ops) {
		return fmt.Errorf("%w: 起始游标 %d 超过操作数 %d", model.ErrInvalidInput, start, len(ops))
	}
	for i := start; i < len(ops); i++ {
		op := ops[i]
		note, vio, err := r.Engine.Apply(ctx, op.Seq, op)
		if err != nil {
			return err
		}
		if vio != nil {
			r.FirstVio = vio
			return nil
		}
		rec := StepRecord{Seq: op.Seq, Type: op.Type, Note: *note}
		rec.Snapshot = r.Engine.Mapping.SnapshotJSON()
		rec.WearHash = r.Engine.SnapshotHash()
		r.Steps = append(r.Steps, rec)
	}
	return nil
}

// Cursor 返回已执行步数（含首违点所在步）。
func (r *Runner) Cursor() int { return len(r.Steps) }
