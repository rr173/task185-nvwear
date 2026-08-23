package model

import "errors"

// 领域错误。
var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrDuplicateMapping  = errors.New("duplicate logical page mapping")
	ErrEraseBadBlock     = errors.New("cannot erase bad block")
	ErrRelocateReserved  = errors.New("cannot relocate to reserved block")
	ErrCheckpointDangling = errors.New("checkpoint references missing log")
	ErrPowerLossNoCheckpoint = errors.New("power loss without recovery checkpoint")
	ErrHotBlock          = errors.New("hot block exceeds wear budget")
	ErrReserveBelowFloor = errors.New("reserved block count below floor")
	ErrConflict          = errors.New("conflict")
	ErrPlanFrozen        = errors.New("plan already published")
	ErrConfigFrozen      = errors.New("config frozen")
)

// 违规类型常量。
const (
	VioDuplicateMapping    = "duplicate_mapping"
	VioEraseBad            = "erase_bad"
	VioRelocateReserved    = "relocate_to_reserved"
	VioCheckpointDangling  = "checkpoint_dangling"
	VioPowerLossNoCheck    = "powerloss_no_checkpoint"
	VioHotBlock            = "hot_block"
	VioReserveBelowFloor   = "reserve_below_floor"
)
