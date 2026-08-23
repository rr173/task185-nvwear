package service

import (
	"context"
	"fmt"
	"time"

	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

// ConfigService 存储配置与块管理。
type ConfigService struct {
	configs  *store.ConfigStore
	blocks   *store.BlockStore
	mappings *store.MappingStore
}

// ConfigCreateInput 创建配置输入。
type ConfigCreateInput struct {
	Name            string
	DieCount        int
	PlanesPerDie    int
	BlocksPerPlane  int
	PagesPerBlock   int
	PageBytes       int
	ReservedBlocks  int
	MaxEraseCycles  int
	WarnThreshold   int
}

// Create 创建存储配置（草案）。
func (s *ConfigService) Create(ctx context.Context, in ConfigCreateInput) (*model.StorageConfig, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: 配置名不能为空", model.ErrInvalidInput)
	}
	c := &model.StorageConfig{
		ID: genID("cfg"), Name: in.Name, Status: model.ConfigDraft,
		DieCount: in.DieCount, PlanesPerDie: in.PlanesPerDie, BlocksPerPlane: in.BlocksPerPlane,
		PagesPerBlock: in.PagesPerBlock, PageBytes: in.PageBytes,
		ReservedBlocks: in.ReservedBlocks, MaxEraseCycles: in.MaxEraseCycles,
		WarnThreshold: in.WarnThreshold, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := geometry.ComputeGeometry(c); err != nil {
		return nil, err
	}
	if err := s.configs.Create(c); err != nil {
		return nil, err
	}
	// 初始化块（默认全部可用）。
	if err := s.initBlocks(ctx, c, nil, nil); err != nil {
		return nil, err
	}
	return c, nil
}

// Get 读取配置。
func (s *ConfigService) Get(ctx context.Context, id string) (*model.StorageConfig, error) {
	return s.configs.Get(id)
}

// List 列出全部配置。
func (s *ConfigService) List(ctx context.Context) ([]model.StorageConfig, error) {
	return s.configs.List()
}

// initBlocks 初始化块视图：可选坏块与保留块。
func (s *ConfigService) initBlocks(ctx context.Context, c *model.StorageConfig, bad []int, badReasons []string) error {
	l, err := geometry.NewLayout(c.DieCount, c.PlanesPerDie, c.BlocksPerPlane,
		c.PagesPerBlock, c.PageBytes, c.ReservedBlocks)
	if err != nil {
		return err
	}
	init, err := geometry.NewBlockInit(bad, badReasons)
	if err != nil {
		return err
	}
	badSet := map[int]bool{}
	for _, b := range init.BadBlocks {
		badSet[b] = true
	}
	// 保留块集合。
	reserveIdx, err := geometry.ReserveSet(l, c.ReservedBlocks)
	if err != nil {
		return err
	}
	reserveSet := map[int]bool{}
	for _, r := range reserveIdx {
		reserveSet[r] = true
	}
	blocks := make([]model.PhysicalBlock, 0, c.TotalBlocks)
	for i := 0; i < c.TotalBlocks; i++ {
		die, plane, blk, _ := l.IndexToLocation(i)
		b := model.PhysicalBlock{
			ConfigID: c.ID, BlockIndex: i, Die: die, Plane: plane, Block: blk,
			Status: model.BlockAvailable, UpdatedAt: time.Now(),
		}
		if badSet[i] {
			b.Status = model.BlockBad
		}
		if reserveSet[i] {
			b.IsReserved = true
			b.Status = model.BlockReserved
		}
		blocks = append(blocks, b)
	}
	if err := s.blocks.ResetAll(c.ID); err != nil {
		return err
	}
	return s.blocks.Insert(blocks)
}

// MarkBad 登记坏块（仅草案/可模拟配置）。
func (s *ConfigService) MarkBad(ctx context.Context, configID string, blockIndex int, reason string) (*model.PhysicalBlock, error) {
	c, err := s.configs.Get(configID)
	if err != nil {
		return nil, err
	}
	if c.Status == model.ConfigFrozen {
		return nil, model.ErrConfigFrozen
	}
	if reason == "" {
		reason = "factory bad block"
	}
	b, err := s.blocks.Get(configID, blockIndex)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BlockBad {
		return b, nil
	}
	if err := s.blocks.UpdateStatus(configID, blockIndex, model.BlockBad, reason); err != nil {
		return nil, err
	}
	return s.blocks.Get(configID, blockIndex)
}

// Reserve 设置保留块。
func (s *ConfigService) Reserve(ctx context.Context, configID string, blockIndex int, reserved bool) (*model.PhysicalBlock, error) {
	c, err := s.configs.Get(configID)
	if err != nil {
		return nil, err
	}
	if c.Status == model.ConfigFrozen {
		return nil, model.ErrConfigFrozen
	}
	b, err := s.blocks.Get(configID, blockIndex)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BlockBad {
		return nil, fmt.Errorf("%w: 坏块 %d 不能设为保留", model.ErrInvalidInput, blockIndex)
	}
	if err := s.blocks.MarkReserved(configID, blockIndex, reserved); err != nil {
		return nil, err
	}
	return s.blocks.Get(configID, blockIndex)
}

// Freeze 冻结配置（此后不可变，证书可引用）。
func (s *ConfigService) Freeze(ctx context.Context, configID string) (*model.StorageConfig, error) {
	c, err := s.configs.Get(configID)
	if err != nil {
		return nil, err
	}
	if c.Status == model.ConfigDraft {
		// 草案需至少登记几何并初始化块后才可冻结。
		if err := s.configs.UpdateStatus(configID, model.ConfigSimulatable); err != nil {
			return nil, err
		}
	}
	if err := s.configs.Freeze(configID); err != nil {
		return nil, err
	}
	return s.configs.Get(configID)
}

// Blocks 列出配置块。
func (s *ConfigService) Blocks(ctx context.Context, configID string) ([]model.PhysicalBlock, error) {
	return s.blocks.List(configID)
}

// InitMapping 初始化逻辑映射（模拟前置：清空旧映射）。
func (s *ConfigService) InitMapping(ctx context.Context, configID string) (int, error) {
	if _, err := s.configs.Get(configID); err != nil {
		return 0, err
	}
	if err := s.mappings.Reset(configID); err != nil {
		return 0, err
	}
	return 0, nil
}
