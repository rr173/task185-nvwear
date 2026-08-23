# BENZHI_README — 评测说明（task185-nvwear）

## 项目定位

非易失存储磨损预算验证服务：登记存储芯片几何与坏块、创建写入/回收操作计划，逐步模拟页写入、块擦除、搬迁与检查点，校验热块均衡、保留块下限与掉电恢复日志完整性，输出可行预算或第一个违反点；通过的计划发布为固件策略证书。

## 评测命令

```bash
# 构建
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...

# 静态检查
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...

# 单元测试
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...

# 端到端自检（唯一判据，退出码 0 = 通过）
go run ./cmd/nvwear --smoke-test
```

## --smoke-test 契约

不启动长驻服务，真实执行以下步骤后以 0 退出：

1. 创建存储配置（4 die × 2 plane × 32 block × 64 page × 4KB，保留 4 块，预算 1000 次擦除），验证容量计算（256 块 / 252 可用 / 16384 页）。
2. 登记坏块（块 0）、冻结配置（frozen）。
3. 创建均衡写入计划（含检查点与掉电标记）→ 编辑中；同哈希计划自动合并。
4. 模拟 → `publishable`；磨损曲线非空。
5. 关闭并重开同一 SQLite 数据库，验证计划状态、哈希与磨损曲线完整恢复。
6. 发布策略证书（冻结配置 + 计划哈希）→ 撤销（revoked）。
7. 负向用例：热块计划（单块擦除 850 次）→ `budget_exceeded`；无检查点掉电 → `recovery_incomplete`；重复映射 → 违规；擦除坏块 → 违规。
8. 统计断言（配置 1、计划 ≥5、证书签发 1、撤销 1、坏块 ≥1、磨损累计 ≥850）。

## Docker 双架构

```bash
# 构建镜像（arm64/amd64 任意平台）
bash build_benzhi_docker.sh my-project linux/amd64
bash build_benzhi_docker.sh my-project linux/arm64

# 运行端到端自检（唯一判据）
docker run --rm my-project:latest --smoke-test
```

镜像内 `CMD ["--smoke-test"]`，直接运行即执行自检并退出；容器只传 `--smoke-test` 标志，不追加程序路径参数。

## API 与数据

- 路由前缀 `/api`，JSON 请求/响应；错误映射：404 not_found、409 违规/状态冲突。
- SQLite 持久化（modernc.org/sqlite v1.52.0），表：storage_configs / physical_blocks / logical_mappings / operation_plans / plan_ops / checkpoints / certificates / wear_entries。
