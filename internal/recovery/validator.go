// Package recovery 验证掉电恢复：检查点引用、恢复日志完整性与重放。
package recovery

import (
	"fmt"

	"task185-nvwear/internal/model"
)

// LogEntry 恢复日志条目（检查点引用的日志段）。
type LogEntry struct {
	Seq    int
	OpType model.OpType
	Note   string
}

// Validator 恢复验证器。
type Validator struct {
	Logs []LogEntry // 完整操作日志
}

// New 构建验证器。
func New(logs []LogEntry) *Validator { return &Validator{Logs: logs} }

// Validate 校验计划：掉电标记必须有前置检查点，检查点不得引用缺失日志。
// 返回首个违反（如有）。
func (v *Validator) Validate(ops []model.PlanOperation, checkpoints map[int]bool) *model.Violation {
	// 检查点引用完整性：每个 checkpoint 引用的游标必须对应已存在日志。
	for cpCursor := range checkpoints {
		if cpCursor > len(ops) {
			return &model.Violation{Step: cpCursor, Type: model.VioCheckpointDangling,
				Message: fmt.Sprintf("检查点游标 %d 超出操作日志长度 %d", cpCursor, len(ops))}
		}
	}
	// 掉电标记必须有前置检查点。
	lastCheckpoint := -1
	for _, op := range ops {
		if op.Type == model.OpCheckpoint {
			lastCheckpoint = op.Seq
		}
		if op.Type == model.OpPowerLoss && lastCheckpoint < 0 {
			return &model.Violation{Step: op.Seq, Type: model.VioPowerLossNoCheck,
				Message: fmt.Sprintf("步骤 %d 掉电前无检查点", op.Seq)}
		}
	}
	return nil
}

// Recoverable 判断是否存在可用检查点。
func Recoverable(ops []model.PlanOperation) (bool, int) {
	last := -1
	for _, op := range ops {
		if op.Type == model.OpCheckpoint {
			last = op.Seq
		}
	}
	return last >= 0, last
}

// Replay 从指定游标重放日志，返回可重放步数与完整性结论。
func (v *Validator) Replay(ops []model.PlanOperation, fromCursor int) (replayed int, integrityOK bool, err error) {
	idx := -1
	for i, op := range ops {
		if op.Seq == fromCursor {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, false, fmt.Errorf("%w: 恢复起点游标 %d 在日志中不存在", model.ErrCheckpointDangling, fromCursor)
	}
	// 重放直到末尾或掉电标记。
	for i := idx + 1; i < len(ops); i++ {
		op := ops[i]
		if op.Type == model.OpPowerLoss {
			// 掉电即停，该步前的日志必须完整存在。
			return replayed, true, nil
		}
		replayed++
	}
	return replayed, true, nil
}
