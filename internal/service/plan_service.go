package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"task185-nvwear/internal/budget"
	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/recovery"
	"task185-nvwear/internal/simulator"
	"task185-nvwear/internal/store"
)

// PlanService 操作计划：创建、追加、模拟、检查点与恢复。
type PlanService struct {
	plans       *store.PlanStore
	ops         *store.OpStore
	checkpoints *store.CheckpointStore
	wear        *store.WearStore
	blocks      *store.BlockStore
	mappings    *store.MappingStore
	configs     *store.ConfigStore
	db          *store.DB
	createMu    *sync.Mutex
}

// PlanCreateInput 创建计划输入。
type PlanCreateInput struct {
	ConfigID string
	Name     string
	Ops      []model.PlanOperation
}

// Create 创建计划（编辑中）。
func (s *PlanService) Create(ctx context.Context, in PlanCreateInput) (*model.OperationPlan, error) {
	if s.createMu != nil {
		s.createMu.Lock()
		defer s.createMu.Unlock()
	}
	if in.Name == "" {
		return nil, fmt.Errorf("%w: 计划名不能为空", model.ErrInvalidInput)
	}
	cfg, err := s.configs.Get(in.ConfigID)
	if err != nil {
		return nil, err
	}
	if cfg.Status != model.ConfigSimulatable && cfg.Status != model.ConfigFrozen {
		return nil, fmt.Errorf("%w: 配置 %s 未处于可模拟状态", model.ErrInvalidState, cfg.Status)
	}
	if len(in.Ops) == 0 {
		return nil, fmt.Errorf("%w: 计划至少包含一个操作", model.ErrInvalidInput)
	}
	hash := hashOps(in.Ops)
	// 同配置同哈希不重复创建。
	if existing, err := s.plans.GetByHash(in.ConfigID, hash); err == nil {
		return existing, nil
	}
	p := &model.OperationPlan{
		ID: genID("plan"), ConfigID: in.ConfigID, Name: in.Name,
		Status: model.PlanEditing, PlanHash: hash, OpCount: len(in.Ops),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.plans.Create(p); err != nil {
		return nil, err
	}
	for i, op := range in.Ops {
		op.Seq = i + 1
		if err := s.ops.Insert(p.ID, op); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// AppendOps 追加操作（仅编辑中）。
func (s *PlanService) AppendOps(ctx context.Context, planID string, ops []model.PlanOperation) (*model.OperationPlan, error) {
	p, err := s.plans.Get(planID)
	if err != nil {
		return nil, err
	}
	if p.Status != model.PlanEditing {
		return nil, fmt.Errorf("%w: 计划 %s 处于 %s，仅编辑中可追加", model.ErrInvalidState, planID, p.Status)
	}
	start, err := s.ops.Count(planID)
	if err != nil {
		return nil, err
	}
	for i, op := range ops {
		op.Seq = start + i + 1
		if err := s.ops.Insert(planID, op); err != nil {
			return nil, err
		}
	}
	// 重新计算哈希。
	all, err := s.ops.List(planID)
	if err != nil {
		return nil, err
	}
	hash := hashOps(all)
	if err := s.plans.UpdateStatus(planID, model.PlanEditing); err != nil {
		return nil, err
	}
	// 更新哈希与操作数。
	_, err = s.db.Sql().Exec(`UPDATE operation_plans SET plan_hash=?, op_count=? WHERE id=?`,
		hash, len(all), planID)
	if err != nil {
		return nil, err
	}
	return s.plans.Get(planID)
}

// Get 读取计划。
func (s *PlanService) Get(ctx context.Context, planID string) (*model.OperationPlan, error) {
	return s.plans.Get(planID)
}

// List 列出配置下计划。
func (s *PlanService) List(ctx context.Context, configID string) ([]model.OperationPlan, error) {
	return s.plans.List(configID)
}

// Ops 读取计划操作序列。
func (s *PlanService) Ops(ctx context.Context, planID string) ([]model.PlanOperation, error) {
	return s.ops.List(planID)
}

// Simulate 执行模拟：验证预算与恢复，输出可行预算或第一个违反点。
func (s *PlanService) Simulate(ctx context.Context, planID string) (*model.OperationPlan, error) {
	p, err := s.plans.Get(planID)
	if err != nil {
		return nil, err
	}
	if p.Status == model.PlanPublishable || p.Status == model.PlanPublished {
		return p, nil
	}
	cfg, err := s.configs.Get(p.ConfigID)
	if err != nil {
		return nil, err
	}
	l, err := geometry.NewLayout(cfg.DieCount, cfg.PlanesPerDie, cfg.BlocksPerPlane,
		cfg.PagesPerBlock, cfg.PageBytes, cfg.ReservedBlocks)
	if err != nil {
		return nil, err
	}
	// 重建块视图与映射（从持久化恢复）。
	blocks, err := s.blocks.List(p.ConfigID)
	if err != nil {
		return nil, err
	}
	rows, err := s.mappings.LatestAll(p.ConfigID)
	if err != nil {
		return nil, err
	}
	allMappings := make([]model.LogicalMapping, 0, len(rows))
	for _, m := range rows {
		allMappings = append(allMappings, m)
	}
	tbl := mapping.FromMappings(allMappings)
	eng := simulator.NewEngine(cfg, l, tbl, blocks)

	ops, err := s.ops.List(planID)
	if err != nil {
		return nil, err
	}
	// 从最近检查点续跑（断点续传）。
	cps, err := s.checkpoints.List(planID)
	if err != nil {
		return nil, err
	}
	start := 0
	if len(cps) > 0 {
		start = cps[len(cps)-1].Cursor
	}

	runner := simulator.NewRunner(eng)
	if err := runner.Run(ctx, ops, start); err != nil {
		return nil, err
	}
	// 操作级违规（重复映射/坏块/保留块/检查点悬空）优先于预算检查。
	if runner.FirstVio != nil {
		if err := s.plans.UpdateSimState(planID, model.PlanBudgetExceeded,
			runner.Cursor(), runner.FirstVio.Step, runner.FirstVio.Message); err != nil {
			return nil, err
		}
		return s.plans.Get(planID)
	}

	// 恢复完整性验证。
	cpSet := map[int]bool{}
	for _, cp := range cps {
		cpSet[cp.Cursor] = true
	}
	recoveryLogs := make([]recovery.LogEntry, 0, len(ops))
	for _, op := range ops {
		recoveryLogs = append(recoveryLogs, recovery.LogEntry{Seq: op.Seq, OpType: op.Type})
	}
	val := recovery.New(recoveryLogs)
	if vio := val.Validate(ops, cpSet); vio != nil {
		if err := s.plans.UpdateSimState(planID, model.PlanRecoveryIncomplete,
			runner.Cursor(), vio.Step, vio.Message); err != nil {
			return nil, err
		}
		return s.plans.Get(planID)
	}

	// 预算校验：热块与保留块下限。
	eval, err := budgetEvaluator(cfg, l)
	if err != nil {
		return nil, err
	}
	sorted := eng.SortedBlocks()
	// 记录磨损曲线（无论预算结果，均保存当前磨损状态供统计与查询）。
	entries := make([]model.WearEntry, 0, len(sorted))
	for _, b := range sorted {
		entries = append(entries, eval.WearEntry(b))
	}
	if err := s.wear.ReplaceAll(planID, entries); err != nil {
		return nil, err
	}
	if vio := eval.Check(runner.Cursor(), sorted); vio != nil {
		if err := s.plans.UpdateSimState(planID, model.PlanBudgetExceeded,
			runner.Cursor(), vio.Step, vio.Message); err != nil {
			return nil, err
		}
		return s.plans.Get(planID)
	}
	if err := s.plans.UpdateSimState(planID, model.PlanPublishable, runner.Cursor(), 0, ""); err != nil {
		return nil, err
	}
	return s.plans.Get(planID)
}

// Checkpoint 在指定游标创建检查点。
func (s *PlanService) Checkpoint(ctx context.Context, planID string, cursor int) (*model.Checkpoint, error) {
	p, err := s.plans.Get(planID)
	if err != nil {
		return nil, err
	}
	ops, err := s.ops.List(planID)
	if err != nil {
		return nil, err
	}
	if cursor < 0 || cursor > len(ops) {
		return nil, fmt.Errorf("%w: 检查点游标 %d 越界", model.ErrInvalidInput, cursor)
	}
	if cursor == 0 {
		return nil, fmt.Errorf("%w: 检查点必须引用已执行步骤", model.ErrInvalidInput)
	}
	blocks, err := s.blocks.List(p.ConfigID)
	if err != nil {
		return nil, err
	}
	snap := geometry.BuildSnapshot(blocks)
	cp := &model.Checkpoint{
		ID: genID("cp"), PlanID: planID, Cursor: cursor,
		SnapshotHash: snap.Hash, LogCount: len(ops), CreatedAt: time.Now(),
	}
	if err := s.checkpoints.Insert(cp); err != nil {
		return nil, err
	}
	return cp, nil
}

// Checkpoints 列出计划检查点。
func (s *PlanService) Checkpoints(ctx context.Context, planID string) ([]model.Checkpoint, error) {
	return s.checkpoints.List(planID)
}

// Recover 执行掉电恢复验证。
func (s *PlanService) Recover(ctx context.Context, planID string) (*model.RecoveryReport, error) {
	ops, err := s.ops.List(planID)
	if err != nil {
		return nil, err
	}
	ok, lastCP := recovery.Recoverable(ops)
	if !ok {
		return &model.RecoveryReport{
			PlanID: planID, Recoverable: false,
			Message: "计划无检查点，掉电后不可恢复",
		}, nil
	}
	logs := make([]recovery.LogEntry, 0, len(ops))
	for _, op := range ops {
		logs = append(logs, recovery.LogEntry{Seq: op.Seq, OpType: op.Type})
	}
	val := recovery.New(logs)
	replayed, integrityOK, err := val.Replay(ops, lastCP)
	if err != nil {
		return &model.RecoveryReport{
			PlanID: planID, Recoverable: false, FromCheckpoint: "",
			IntegrityOK: false, Message: err.Error(),
		}, nil
	}
	cp, _ := s.checkpoints.Latest(planID)
	rep := &model.RecoveryReport{
		PlanID: planID, Recoverable: true, ReplaySteps: len(ops) - lastCP,
		Replayed: replayed, IntegrityOK: integrityOK, Message: "从检查点重放成功",
	}
	if cp != nil {
		rep.FromCheckpoint = cp.ID
	}
	return rep, nil
}

// Wear 读取磨损曲线。
func (s *PlanService) Wear(ctx context.Context, planID string) ([]model.WearEntry, error) {
	return s.wear.List(planID)
}

// budgetEvaluator 构建预算评估器。
func budgetEvaluator(cfg *model.StorageConfig, l geometry.Layout) (*budget.Evaluator, error) {
	return budget.New(cfg.MaxEraseCycles, cfg.WarnThreshold, cfg.ReservedBlocks, l.UsableBlocks(), 81)
}

// hashOps 计算操作序列哈希。
func hashOps(ops []model.PlanOperation) string {
	h := sha256.New()
	for _, op := range ops {
		fmt.Fprintf(h, "%d|%s|%d|%d|%d|%d;", op.Seq, op.Type, op.LPN, op.SrcPPN, op.DestPPN, op.BlockIndex)
	}
	return hex.EncodeToString(h.Sum(nil))
}
