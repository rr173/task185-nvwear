// Package model 定义非易失存储磨损预算验证服务的核心实体与状态机。
package model

import "time"

// 存储配置状态机：draft → simulatable → frozen；draft → conflict → simulatable。
const (
	ConfigDraft       = "draft"        // 草案：几何未完整
	ConfigSimulatable = "simulatable"  // 可模拟：几何与坏块完整，可接收计划
	ConfigConflict    = "conflict"     // 几何冲突：容量计算不一致
	ConfigFrozen      = "frozen"       // 已冻结：配置不可变，证书引用其快照
)

// 物理块状态机：available → worn → bad → isolated；available → reserved。
const (
	BlockAvailable = "available" // 可用：可承载数据与磨损
	BlockReserved  = "reserved"  // 保留：仅作替换/搬迁目标，不参与常规写入
	BlockWorn      = "worn"      // 磨损预警：擦除计数接近预算
	BlockBad      = "bad"        // 坏块：擦除/写入失败
	BlockIsolated = "isolated"   // 已隔离：从可用容量中剔除
)

// 操作计划状态机：editing → simulating → publishable / budget_exceeded / recovery_incomplete。
const (
	PlanEditing             = "editing"              // 编辑中：可追加操作
	PlanSimulating          = "simulating"           // 模拟中：正在逐步骤执行
	PlanPublishable         = "publishable"          // 可发布：预算与恢复均通过
	PlanBudgetExceeded      = "budget_exceeded"      // 预算超限：存在违反点
	PlanRecoveryIncomplete  = "recovery_incomplete"  // 恢复不完整：掉电后无法从检查点重放
	PlanPublished           = "published"            // 已发布：已签发策略证书
)

// 策略证书状态机：draft → published → revoked。
const (
	CertDraft    = "draft"     // 草案
	CertPublished = "published" // 已发布
	CertRevoked  = "revoked"   // 已撤销
)

// OpType 操作类型。
type OpType string

const (
	OpWrite     OpType = "write"     // 页写入：将逻辑页写入物理页
	OpErase     OpType = "erase"     // 块擦除：整块擦除并清空映射
	OpRelocate  OpType = "relocate"  // 搬迁：从源物理页搬到目标物理页
	OpCheckpoint OpType = "checkpoint" // 检查点：记录当前游标与映射快照
	OpPowerLoss OpType = "powerloss" // 掉电标记：标注计划中的掉电事件
)

// StorageConfig 存储配置：芯片几何、坏块与状态。
type StorageConfig struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	DieCount     int       `json:"die_count"`     // 晶粒数
	PlanesPerDie int       `json:"planes_per_die"` // 每 die 平面数
	BlocksPerPlane int    `json:"blocks_per_plane"` // 每平面块数
	PagesPerBlock int     `json:"pages_per_block"` // 每块页数
	PageBytes    int       `json:"page_bytes"`    // 每页字节数
	ReservedBlocks int    `json:"reserved_blocks"` // 保留块数量（从总块中划出）
	MaxEraseCycles int    `json:"max_erase_cycles"` // 单块磨损预算（擦除次数上限）
	WarnThreshold  int    `json:"warn_threshold"`  // 磨损预警阈值（占 MaxEraseCycles 百分比）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	FrozenAt     *time.Time `json:"frozen_at,omitempty"`
	// 几何摘要（不可变派生值）
	TotalBlocks int `json:"total_blocks"`
	UsableBlocks int `json:"usable_blocks"`
	TotalPages  int `json:"total_pages"`
	TotalBytes  int64 `json:"total_bytes"`
}

// PhysicalBlock 物理块运行时状态。
type PhysicalBlock struct {
	ConfigID     string `json:"config_id"`
	BlockIndex   int    `json:"block_index"`   // 全局线性块号 0..TotalBlocks-1
	Die          int    `json:"die"`
	Plane        int    `json:"plane"`
	Block        int    `json:"block"`
	Status       string `json:"status"`
	EraseCount   int    `json:"erase_count"`
	IsReserved   bool   `json:"is_reserved"`
	BadReason    string `json:"bad_reason,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LogicalMapping 逻辑页到物理页的映射记录。
type LogicalMapping struct {
	ConfigID     string `json:"config_id"`
	LPN          int    `json:"lpn"`           // 逻辑页号
	PPN          int    `json:"ppn"`           // 物理页号（全局线性）
	BlockIndex   int    `json:"block_index"`   // 所在块
	PageInBlock  int    `json:"page_in_block"` // 块内页号
	Version      int    `json:"version"`       // 映射版本（同 LPN 覆盖递增）
	CreatedAt    time.Time `json:"created_at"`
}

// PlanOperation 计划中的单个操作。
type PlanOperation struct {
	Seq        int    `json:"seq"`
	Type       OpType `json:"type"`
	LPN        int    `json:"lpn,omitempty"`       // write/relocate 源逻辑页
	SrcPPN     int    `json:"src_ppn,omitempty"`   // relocate 源物理页
	DestPPN    int    `json:"dest_ppn,omitempty"`  // write/relocate 目标物理页
	BlockIndex int    `json:"block_index,omitempty"` // erase 目标块
	Note       string `json:"note,omitempty"`
}

// OperationPlan 操作计划。
type OperationPlan struct {
	ID          string    `json:"id"`
	ConfigID    string    `json:"config_id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	PlanHash    string    `json:"plan_hash"`     // 操作序列哈希：同哈希不重复模拟
	OpCount     int       `json:"op_count"`
	SimCursor   int       `json:"sim_cursor"`    // 已执行到第几步
	ViolationStep int     `json:"violation_step,omitempty"` // 第一个违反步骤
	ViolationMsg  string  `json:"violation_msg,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SimulatedAt *time.Time `json:"simulated_at,omitempty"`
}

// Checkpoint 检查点：模拟游标 + 映射快照哈希 + 块状态摘要。
type Checkpoint struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	Cursor      int       `json:"cursor"`       // 对应模拟到第几步
	SnapshotHash string   `json:"snapshot_hash"` // 映射+块状态快照哈希
	LogCount    int       `json:"log_count"`     // 检查点引用的恢复日志条数
	CreatedAt   time.Time `json:"created_at"`
}

// WearEntry 单块磨损曲线点。
type WearEntry struct {
	BlockIndex int `json:"block_index"`
	EraseCount int `json:"erase_count"`
	WearPct    int `json:"wear_pct"`   // 磨损百分比 0-100
	Tier       string `json:"tier"`    // healthy/warning/critical
	IsReserved bool   `json:"is_reserved"`
}

// Violation 违反点。
type Violation struct {
	Step    int    `json:"step"`
	Type    string `json:"type"`    // duplicate_mapping / erase_bad / relocate_to_reserved / checkpoint_dangling / powerloss_no_checkpoint / hot_block / reserve_below_floor
	Message string `json:"message"`
	Block   int    `json:"block,omitempty"`
}

// StrategyCertificate 策略证书：冻结配置、初始映射与操作序列。
type StrategyCertificate struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	ConfigID    string    `json:"config_id"`
	Status      string    `json:"status"`
	Name        string    `json:"name"`
	ConfigSnapshot string `json:"config_snapshot"` // 冻结配置 JSON
	MappingSnapshot string `json:"mapping_snapshot"` // 初始映射 JSON
	PlanHash     string   `json:"plan_hash"`
	IssuedAt    time.Time `json:"issued_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RevokeReason string   `json:"revoke_reason,omitempty"`
}

// RecoveryReport 掉电恢复报告。
type RecoveryReport struct {
	PlanID       string    `json:"plan_id"`
	Recoverable  bool      `json:"recoverable"`
	FromCheckpoint string  `json:"from_checkpoint,omitempty"`
	ReplaySteps  int       `json:"replay_steps"`
	Replayed     int       `json:"replayed"`
	IntegrityOK  bool      `json:"integrity_ok"`
	Message      string    `json:"message"`
}
