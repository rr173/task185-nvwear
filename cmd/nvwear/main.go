// task185-nvwear 非易失存储磨损预算验证服务入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"task185-nvwear/internal/httpapi"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/service"
	"task185-nvwear/internal/store"
)

var (
	addr   = flag.String("addr", ":8080", "HTTP listen address")
	dbPath = flag.String("db", "task185-nvwear.db", "SQLite database path")
	smoke  = flag.Bool("smoke-test", false, "run offline smoke test and exit")
)

func main() {
	flag.Parse()

	if *smoke {
		tmp := *dbPath + ".smoke"
		_ = os.Remove(tmp)
		_ = os.Remove(tmp + "-shm")
		_ = os.Remove(tmp + "-wal")
		db, err := store.Open(tmp)
		if err != nil {
			log.Fatalf("open smoke db: %v", err)
		}
		if err := runSmoke(db); err != nil {
			_ = db.Close()
			log.Fatalf("smoke-test failed: %v", err)
		}
		_ = db.Close()
		_ = os.Remove(tmp)
		_ = os.Remove(tmp + "-shm")
		_ = os.Remove(tmp + "-wal")
		fmt.Println("smoke-test passed")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app := service.New(db)
	srv := httpapi.NewServer(app)
	log.Printf("task185-nvwear listening on %s (db=%s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// runSmoke 端到端自检：
//  1. 创建存储配置（几何 + 保留块）并冻结；
//  2. 初始化映射，登记坏块；
//  3. 创建均衡写入计划（含检查点与掉电标记）并模拟 → publishable；
//  4. 关闭并重开数据库验证重启恢复：计划状态、磨损曲线、检查点保持；
//  5. 发布策略证书并验证冻结快照；撤销证书；
//  6. 负向用例：热块计划预算超限、无检查点掉电恢复不完整、重复映射被拒绝、坏块擦除被拒绝。
func runSmoke(db *store.DB) error {
	ctx := context.Background()
	app := service.New(db)

	// 1. 创建配置：4 die × 2 plane × 32 block × 64 page × 4KB，保留 4 块，预算 1000 次擦除。
	cfg, err := app.Configs.Create(ctx, service.ConfigCreateInput{
		Name: "smoke-ssd", DieCount: 4, PlanesPerDie: 2, BlocksPerPlane: 32,
		PagesPerBlock: 64, PageBytes: 4096, ReservedBlocks: 4,
		MaxEraseCycles: 1000, WarnThreshold: 80,
	})
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	if cfg.Status != model.ConfigDraft {
		return fmt.Errorf("新配置应为 draft，实际 %s", cfg.Status)
	}
	if cfg.TotalBlocks != 4*2*32 || cfg.UsableBlocks != 4*2*32-4 || cfg.TotalPages != 4*2*32*64 {
		return fmt.Errorf("几何容量计算错误: %+v", cfg)
	}
	// 登记坏块。
	if _, err := app.Configs.MarkBad(ctx, cfg.ID, 0, "factory defect"); err != nil {
		return fmt.Errorf("mark bad: %w", err)
	}
	// 冻结配置。
	frozen, err := app.Configs.Freeze(ctx, cfg.ID)
	if err != nil {
		return fmt.Errorf("freeze config: %w", err)
	}
	if frozen.Status != model.ConfigFrozen {
		return fmt.Errorf("配置应已冻结，实际 %s", frozen.Status)
	}
	if _, err := app.Configs.InitMapping(ctx, cfg.ID); err != nil {
		return fmt.Errorf("init mapping: %w", err)
	}

	// 2. 均衡写入计划：写 LPN 到各非保留块（跳过坏块 0 与保留块），含检查点与掉电。
	//    可用块 = 254（256 - 4 保留 - 1 坏 = 251 可用；为简单，使用块 1..60 轮转写入）。
	//    预算热块阈值 80% × 1000 = 800 次擦除；此计划少量写入不会触发。
	ops := []model.PlanOperation{
		// 先写后擦，构造可控磨损。
		{Type: model.OpWrite, LPN: 1, DestPPN: pageOf(1, 0)},
		{Type: model.OpWrite, LPN: 2, DestPPN: pageOf(2, 0)},
		{Type: model.OpErase, BlockIndex: 1},
		{Type: model.OpWrite, LPN: 1, DestPPN: pageOf(1, 1)},
		{Type: model.OpCheckpoint, Note: "cp after step 5"},
		{Type: model.OpWrite, LPN: 3, DestPPN: pageOf(3, 0)},
		{Type: model.OpPowerLoss, Note: "power cut"},
		{Type: model.OpWrite, LPN: 4, DestPPN: pageOf(4, 0)},
	}
	p, err := app.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-balanced", Ops: ops,
	})
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	if p.Status != model.PlanEditing {
		return fmt.Errorf("计划应处于编辑中，实际 %s", p.Status)
	}
	// 同哈希不重复创建。
	dup, err := app.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-balanced-dup", Ops: ops,
	})
	if err != nil {
		return fmt.Errorf("duplicate create: %w", err)
	}
	if dup.ID != p.ID {
		return fmt.Errorf("同哈希计划应合并，%s != %s", dup.ID, p.ID)
	}

	// 3. 模拟 → publishable。
	sim, err := app.Plans.Simulate(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("simulate: %w", err)
	}
	if sim.Status != model.PlanPublishable {
		return fmt.Errorf("计划应为 publishable，实际 %s (violation=%s)",
			sim.Status, sim.ViolationMsg)
	}
	wear, err := app.Plans.Wear(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("wear: %w", err)
	}
	if len(wear) == 0 {
		return fmt.Errorf("磨损曲线为空")
	}

	// 4. 关闭并重开数据库验证重启恢复。
	if err := db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	db2, err := store.Open(db.Path)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	app2 := service.New(db2)
	p2, err := app2.Plans.Get(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("restart read plan: %w", err)
	}
	if p2.Status != model.PlanPublishable {
		return fmt.Errorf("重启后计划状态不一致: %s", p2.Status)
	}
	if p2.PlanHash != p.PlanHash {
		return fmt.Errorf("重启后计划哈希不一致")
	}
	wear2, err := app2.Plans.Wear(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("restart wear: %w", err)
	}
	if len(wear2) != len(wear) {
		return fmt.Errorf("重启后磨损曲线不一致")
	}

	// 5. 发布证书并撤销。
	cert, err := app2.Certs.Issue(ctx, p.ID, "smoke-policy")
	if err != nil {
		return fmt.Errorf("issue cert: %w", err)
	}
	if cert.Status != model.CertPublished {
		return fmt.Errorf("证书应已发布，实际 %s", cert.Status)
	}
	if cert.ConfigSnapshot == "" || cert.PlanHash != p.PlanHash {
		return fmt.Errorf("证书未冻结配置/计划哈希")
	}
	revoked, err := app2.Certs.Revoke(ctx, cert.ID, "geometry updated")
	if err != nil {
		return fmt.Errorf("revoke cert: %w", err)
	}
	if revoked.Status != model.CertRevoked {
		return fmt.Errorf("证书应已撤销，实际 %s", revoked.Status)
	}

	// 6a. 热块计划 → 预算超限：对单块反复擦除至超阈值。
	hotOps := make([]model.PlanOperation, 0, 900)
	for i := 0; i < 850; i++ {
		hotOps = append(hotOps, model.PlanOperation{Type: model.OpErase, BlockIndex: 2})
	}
	hotPlan, err := app2.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-hot", Ops: hotOps,
	})
	if err != nil {
		return fmt.Errorf("create hot plan: %w", err)
	}
	hotSim, err := app2.Plans.Simulate(ctx, hotPlan.ID)
	if err != nil {
		return fmt.Errorf("simulate hot plan: %w", err)
	}
	if hotSim.Status != model.PlanBudgetExceeded {
		return fmt.Errorf("热块计划应为 budget_exceeded，实际 %s", hotSim.Status)
	}

	// 6b. 无检查点的掉电 → 恢复不完整。
	lossOps := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 10, DestPPN: pageOf(10, 0)},
		{Type: model.OpPowerLoss, Note: "no cp before"},
	}
	lossPlan, err := app2.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-loss", Ops: lossOps,
	})
	if err != nil {
		return fmt.Errorf("create loss plan: %w", err)
	}
	lossSim, err := app2.Plans.Simulate(ctx, lossPlan.ID)
	if err != nil {
		return fmt.Errorf("simulate loss plan: %w", err)
	}
	if lossSim.Status != model.PlanRecoveryIncomplete {
		return fmt.Errorf("无检查点掉电计划应为 recovery_incomplete，实际 %s", lossSim.Status)
	}
	rec, err := app2.Plans.Recover(ctx, lossPlan.ID)
	if err != nil {
		return fmt.Errorf("recover loss plan: %w", err)
	}
	if rec.Recoverable {
		return fmt.Errorf("无检查点不应可恢复: %+v", rec)
	}

	// 6c. 重复映射被拒绝。
	dupOps := []model.PlanOperation{
		{Type: model.OpWrite, LPN: 5, DestPPN: pageOf(5, 0)},
		{Type: model.OpWrite, LPN: 5, DestPPN: pageOf(6, 0)},
	}
	dupPlan, err := app2.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-dup-mapping", Ops: dupOps,
	})
	if err != nil {
		return fmt.Errorf("create dup plan: %w", err)
	}
	dupSim, err := app2.Plans.Simulate(ctx, dupPlan.ID)
	if err != nil {
		return fmt.Errorf("simulate dup plan: %w", err)
	}
	if dupSim.Status != model.PlanBudgetExceeded || dupSim.ViolationMsg == "" {
		return fmt.Errorf("重复映射应产生违规，状态=%s", dupSim.Status)
	}

	// 6d. 擦除坏块被拒绝。
	badEraseOps := []model.PlanOperation{
		{Type: model.OpErase, BlockIndex: 0}, // 块 0 为坏块
	}
	bePlan, err := app2.Plans.Create(ctx, service.PlanCreateInput{
		ConfigID: cfg.ID, Name: "smoke-bad-erase", Ops: badEraseOps,
	})
	if err != nil {
		return fmt.Errorf("create bad erase plan: %w", err)
	}
	beSim, err := app2.Plans.Simulate(ctx, bePlan.ID)
	if err != nil {
		return fmt.Errorf("simulate bad erase plan: %w", err)
	}
	if beSim.Status != model.PlanBudgetExceeded {
		return fmt.Errorf("擦除坏块应产生违规，状态=%s", beSim.Status)
	}

	// 7. 统计断言。
	stats, err := app2.Stats.Collect(ctx)
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	if stats.ConfigCount != 1 || stats.PlanCount < 5 || stats.CertIssued != 1 ||
		stats.CertRevoked != 1 || stats.BadBlockCount < 1 || stats.TotalEraseCount < 850 {
		return fmt.Errorf("统计异常: %+v", stats)
	}

	_ = db2.Close()
	return nil
}

// pageOf 由块号与块内页号计算全局物理页号。
func pageOf(block, pageInBlock int) int {
	// 与几何一致：64 页/块。
	return block*64 + pageInBlock
}
