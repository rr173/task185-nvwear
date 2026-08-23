// Package geometry 定义存储芯片几何：晶粒/平面/块/页布局、容量与寻址。
package geometry

import (
	"fmt"

	"task185-nvwear/internal/model"
)

// Layout 芯片几何布局。
type Layout struct {
	DieCount      int
	PlanesPerDie  int
	BlocksPerPlane int
	PagesPerBlock int
	PageBytes     int
	ReservedBlocks int
}

// NewLayout 构造几何布局，校验非零与一致性。
func NewLayout(die, planes, blocks, pages, pageBytes, reserved int) (Layout, error) {
	if die <= 0 || planes <= 0 || blocks <= 0 || pages <= 0 || pageBytes <= 0 {
		return Layout{}, fmt.Errorf("%w: 几何尺寸必须为正", model.ErrInvalidInput)
	}
	if reserved < 0 {
		return Layout{}, fmt.Errorf("%w: 保留块数不能为负", model.ErrInvalidInput)
	}
	total := die * planes * blocks
	if reserved >= total {
		return Layout{}, fmt.Errorf("%w: 保留块 %d 不得大于等于总块 %d", model.ErrInvalidInput, reserved, total)
	}
	return Layout{
		DieCount: die, PlanesPerDie: planes, BlocksPerPlane: blocks,
		PagesPerBlock: pages, PageBytes: pageBytes, ReservedBlocks: reserved,
	}, nil
}

// TotalBlocks 总块数。
func (l Layout) TotalBlocks() int { return l.DieCount * l.PlanesPerDie * l.BlocksPerPlane }

// UsableBlocks 可用块数（总块 - 保留块）。
func (l Layout) UsableBlocks() int { return l.TotalBlocks() - l.ReservedBlocks }

// TotalPages 总页数。
func (l Layout) TotalPages() int { return l.TotalBlocks() * l.PagesPerBlock }

// TotalBytes 总容量字节。
func (l Layout) TotalBytes() int64 { return int64(l.TotalPages()) * int64(l.PageBytes) }

// PageToBlock 由全局物理页号推导块号与块内页号。
func (l Layout) PageToBlock(ppn int) (blockIndex, pageInBlock int, err error) {
	if ppn < 0 || ppn >= l.TotalPages() {
		return 0, 0, fmt.Errorf("%w: 物理页号 %d 越界 [0,%d)", model.ErrInvalidInput, ppn, l.TotalPages())
	}
	return ppn / l.PagesPerBlock, ppn % l.PagesPerBlock, nil
}

// BlockToPage 由块号推导块首物理页号。
func (l Layout) BlockToPage(blockIndex int) (int, error) {
	if blockIndex < 0 || blockIndex >= l.TotalBlocks() {
		return 0, fmt.Errorf("%w: 块号 %d 越界 [0,%d)", model.ErrInvalidInput, blockIndex, l.TotalBlocks())
	}
	return blockIndex * l.PagesPerBlock, nil
}

// IndexToLocation 将全局块号拆解为 die/plane/block。
func (l Layout) IndexToLocation(blockIndex int) (die, plane, block int, err error) {
	if blockIndex < 0 || blockIndex >= l.TotalBlocks() {
		return 0, 0, 0, fmt.Errorf("%w: 块号 %d 越界", model.ErrInvalidInput, blockIndex)
	}
	planeStride := l.BlocksPerPlane
	dieStride := l.PlanesPerDie * l.BlocksPerPlane
	die = blockIndex / dieStride
	rest := blockIndex % dieStride
	plane = rest / planeStride
	block = rest % planeStride
	return
}

// LocationToIndex 将 die/plane/block 组合为全局块号。
func (l Layout) LocationToIndex(die, plane, block int) (int, error) {
	if die < 0 || die >= l.DieCount || plane < 0 || plane >= l.PlanesPerDie ||
		block < 0 || block >= l.BlocksPerPlane {
		return 0, fmt.Errorf("%w: 位置 (%d,%d,%d) 越界", model.ErrInvalidInput, die, plane, block)
	}
	return die*l.PlanesPerDie*l.BlocksPerPlane + plane*l.BlocksPerPlane + block, nil
}

// ComputeGeometry 计算派生容量并填充配置对象。
func ComputeGeometry(c *model.StorageConfig) error {
	l, err := NewLayout(c.DieCount, c.PlanesPerDie, c.BlocksPerPlane,
		c.PagesPerBlock, c.PageBytes, c.ReservedBlocks)
	if err != nil {
		return err
	}
	if c.MaxEraseCycles <= 0 {
		return fmt.Errorf("%w: 磨损预算必须为正", model.ErrInvalidInput)
	}
	if c.WarnThreshold <= 0 || c.WarnThreshold > 100 {
		return fmt.Errorf("%w: 预警阈值须在 1-100", model.ErrInvalidInput)
	}
	c.TotalBlocks = l.TotalBlocks()
	c.UsableBlocks = l.UsableBlocks()
	c.TotalPages = l.TotalPages()
	c.TotalBytes = l.TotalBytes()
	return nil
}
