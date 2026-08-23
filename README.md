# task185-nvwear — 非易失存储磨损预算验证服务

基于 REQ-20260823-048 生成。嵌入式存储工程师登记芯片几何（晶粒/平面/块/页）、坏块与保留块，提交逻辑写入与垃圾回收操作计划；服务逐步模拟页写入、块擦除、搬迁与检查点，校验热块均衡、保留块下限与掉电恢复日志完整性，输出可行磨损预算或第一个违反点；通过的计划发布为固件策略证书。

## 业务闭环

1. 登记存储配置（die/plane/block/page 几何 + 保留块 + 磨损预算），计算容量并初始化块视图。
2. 登记坏块、设置保留块，冻结配置（此后不可变，证书引用其快照）。
3. 创建操作计划（write / erase / relocate / checkpoint / powerloss 序列），同哈希计划自动合并。
4. 运行模拟：逐步骤执行操作，检测重复映射、擦除坏块、迁移到保留块等操作级违规。
5. 恢复验证：检查点引用完整性 + 掉电标记必须有前置检查点。
6. 预算校验：热块磨损阈值（默认 80%）、保留块数量下限。
7. 通过的计划 → `publishable` → 发布策略证书（冻结配置 + 初始映射 + 操作序列哈希）。
8. 发布后标记块损坏 → 新计划重新评估，旧证书保持原条件（可撤销）。

## 状态机

- 存储配置：`draft → simulatable → frozen`
- 物理块：`available → worn → bad → isolated`；`available → reserved`
- 操作计划：`editing → simulating → publishable / budget_exceeded / recovery_incomplete → published`
- 策略证书：`draft → published → revoked`

## 标准命令

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...

# 静态检查
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...

# 单元测试
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...

# 端到端自检（关闭重开数据库验证重启恢复）
go run ./cmd/nvwear --smoke-test

# 启动服务
go run ./cmd/nvwear --addr :8080 --db nvwear.db
```

## API 一览（前缀 /api）

| 能力 | 入口 |
|---|---|
| 创建/列表/读取配置 | `POST /api/configs` `GET /api/configs` `GET /api/configs/{id}` |
| 冻结配置、初始化映射 | `POST /api/configs/{id}/freeze` `POST /api/configs/{id}/mapping/init` |
| 块清单、登记坏块、设置保留块 | `GET /api/configs/{id}/blocks` `POST .../blocks/{index}/bad` `POST .../blocks/{index}/reserve` |
| 创建/列表/读取计划 | `POST /api/plans` `GET /api/plans?config_id=` `GET /api/plans/{id}` |
| 追加/读取操作 | `POST /api/plans/{id}/ops` `GET /api/plans/{id}/ops` |
| 模拟、磨损曲线 | `POST /api/plans/{id}/simulate` `GET /api/plans/{id}/wear` |
| 检查点、掉电恢复 | `POST /api/plans/{id}/checkpoints` `GET /api/plans/{id}/checkpoints` `POST /api/plans/{id}/recover` |
| 证书发布/列表/读取/撤销 | `POST /api/certificates` `GET /api/certificates?config_id=` `GET /api/certificates/{id}` `POST /api/certificates/{id}/revoke` |
| 健康/统计 | `GET /api/health` `GET /api/stats` |

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。表：`storage_configs`、`physical_blocks`、`logical_mappings`、`operation_plans`、`plan_ops`、`checkpoints`、`certificates`、`wear_entries`。服务重启后从最近检查点继续模拟；同一计划哈希不重复模拟；证书冻结配置、初始映射与操作序列。
