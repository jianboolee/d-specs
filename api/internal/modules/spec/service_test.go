package spec

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/donktech/d-specs/internal/ai/ops"
	"github.com/donktech/d-specs/internal/infrastructure/database"
	"github.com/donktech/d-specs/internal/infrastructure/idgen"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 集成测试，直连数据库。数据库不可用时自动跳过，不影响纯逻辑单测。
//
// 默认连本地开发库，可用 DS_TEST_DATABASE_URL 覆盖。

const defaultTestDSN = "postgresql://postgres:postgres@localhost:5432/dspecs?sslmode=disable"

func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("DS_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.NewGormDB(ctx, dsn, false)
	if err != nil {
		t.Skipf("数据库不可用，跳过集成测试: %v", err)
	}
	db.Logger = db.Logger.LogMode(logger.Silent)
	return db
}

// createTestUser 建一个测试用户，并在用例结束后清理其全部数据
func createTestUser(t *testing.T, db *gorm.DB, label string) string {
	t.Helper()

	id := idgen.New()
	err := db.Exec(
		`INSERT INTO users (id, email, name, password_hash) VALUES (?, ?, ?, ?)`,
		id, id+"@test.local", label, "not-a-real-hash",
	).Error
	if err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	t.Cleanup(func() {
		// spec_items / spec_revisions 都是 ON DELETE CASCADE，
		// 先删表即可连带清干净
		db.Exec(`DELETE FROM spec_tables WHERE user_id = ?`, id)
		db.Exec(`DELETE FROM users WHERE id = ?`, id)
	})

	return id
}

func newTestService(t *testing.T) (*Service, *Repository, *gorm.DB) {
	t.Helper()
	db := testDB(t)
	repo := NewRepository(db)
	return NewService(repo), repo, db
}

func applyItems(t *testing.T, svc *Service, userID, tableID string, summary string, itemOps ...ops.Op) *ApplyResult {
	t.Helper()

	table, err := svc.repo.GetTable(context.Background(), userID, tableID)
	if err != nil {
		t.Fatalf("读取配置表失败: %v", err)
	}
	base := table.CurrentRevision

	res, err := svc.Apply(context.Background(), userID, tableID, ApplyRequest{
		BaseRevision: &base,
		Source:       SourceUser,
		Summary:      summary,
		Ops:          itemOps,
	})
	if err != nil {
		t.Fatalf("应用变更失败: %v", err)
	}
	return res
}

// ==================== 租户隔离 ====================

func TestCrossUserIsolation(t *testing.T) {
	svc, repo, db := newTestService(t)
	ctx := context.Background()

	owner := createTestUser(t, db, "表主人")
	intruder := createTestUser(t, db, "越权者")

	// owner 建表并写入内容
	created, err := svc.Create(ctx, owner, CreateRequest{Title: "owner 的手机配置表"})
	if err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tableID := created.Table.ID

	applyItems(t, svc, owner, tableID, "初始参数",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "电池容量", Value: "5000mAh", ValueType: ops.TypeNumber, GroupName: "续航"}},
	)

	// 下面每一项都必须失败，且返回「不存在」而不是「无权限」——
	// 否则等于告诉攻击者这个 id 是真实存在的
	t.Run("读取他人表详情", func(t *testing.T) {
		if _, err := svc.Detail(ctx, intruder, tableID); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("读取他人表的参数项", func(t *testing.T) {
		if _, _, err := repo.GetTableWithItems(ctx, intruder, tableID); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("修改他人表", func(t *testing.T) {
		title := "被篡改了"
		if _, err := svc.UpdateMeta(ctx, intruder, tableID, UpdateRequest{Title: &title}); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("删除他人表", func(t *testing.T) {
		if err := svc.Delete(ctx, intruder, tableID); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("对他人表应用变更", func(t *testing.T) {
		base := 0
		_, err := svc.Apply(ctx, intruder, tableID, ApplyRequest{
			BaseRevision: &base,
			Ops: []ops.Op{{
				Op:   ops.OpAddItem,
				Item: &ops.Draft{Name: "注入的脏数据", Value: "1"},
			}},
		})
		if !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("读取他人表的版本列表", func(t *testing.T) {
		if _, err := svc.Revisions(ctx, intruder, tableID, 10); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("读取他人表的版本快照", func(t *testing.T) {
		if _, err := repo.GetRevisionItems(ctx, intruder, tableID, 0); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("复制他人表", func(t *testing.T) {
		if _, err := svc.Duplicate(ctx, intruder, tableID, DuplicateRequest{Title: "偷来的副本"}); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("恢复到他人表的版本", func(t *testing.T) {
		if _, err := svc.Restore(ctx, intruder, tableID, 0); !errors.Is(err, ErrTableNotFound) {
			t.Errorf("期望 ErrTableNotFound，实际 %v", err)
		}
	})

	t.Run("列表不包含他人表", func(t *testing.T) {
		list, err := svc.List(ctx, intruder, "", 1, 100)
		if err != nil {
			t.Fatalf("列表查询失败: %v", err)
		}
		for _, item := range list.Items {
			if item.ID == tableID {
				t.Fatal("列表里出现了他人的配置表")
			}
		}
	})

	t.Run("原表未被上述操作破坏", func(t *testing.T) {
		detail, err := svc.Detail(ctx, owner, tableID)
		if err != nil {
			t.Fatalf("owner 应仍能读取自己的表: %v", err)
		}
		if detail.Table.Title != "owner 的手机配置表" {
			t.Errorf("标题被改动: %q", detail.Table.Title)
		}
		if len(detail.Items) != 1 {
			t.Errorf("参数项数量被改动: %d", len(detail.Items))
		}
	})
}

// ==================== 变更与版本 ====================

func TestApplyPersistsItemsAndRevision(t *testing.T) {
	svc, repo, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "变更测试")

	created, err := svc.Create(ctx, userID, CreateRequest{Title: "测试表"})
	if err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tableID := created.Table.ID

	// 建表时应已写入 revision 0，让「恢复到初始状态」有统一起点
	revs, err := repo.ListRevisions(ctx, userID, tableID, 10)
	if err != nil {
		t.Fatalf("读取版本失败: %v", err)
	}
	if len(revs) != 1 || revs[0].Revision != 0 {
		t.Fatalf("期望初始有 revision 0，实际 %+v", revs)
	}

	// 一次应用两条 op
	res := applyItems(t, svc, userID, tableID, "加两个参数",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "电池容量", Value: "5000mAh", ValueType: ops.TypeNumber, GroupName: "续航"}},
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "无线充电", Value: "支持", ValueType: ops.TypeBool, GroupName: "续航"}},
	)

	if res.Revision != 1 {
		t.Errorf("期望版本号 1，实际 %d", res.Revision)
	}
	if len(res.Changes) != 2 {
		t.Errorf("期望 2 条变更，实际 %d", len(res.Changes))
	}

	// 参数项应落库，值按原样保存（单位已在值里）
	_, items, err := repo.GetTableWithItems(ctx, userID, tableID)
	if err != nil {
		t.Fatalf("读取参数项失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 个参数项，实际 %d", len(items))
	}

	var battery *ops.Item
	for i := range items {
		if items[i].Name == "电池容量" {
			battery = &items[i]
		}
	}
	if battery == nil {
		t.Fatal("找不到「电池容量」")
	}
	if battery.Value != "5000mAh" {
		t.Errorf("参数值期望 5000mAh，实际 %q", battery.Value)
	}

	// value_num 冗余列应被填充，供后续排序与区间筛选使用
	var num *float64
	if err := db.Raw(`SELECT value_num FROM spec_items WHERE id = ?`, battery.ID).Scan(&num).Error; err != nil {
		t.Fatalf("读取 value_num 失败: %v", err)
	}
	if num == nil || *num != 5000 {
		t.Errorf("value_num 期望 5000，实际 %v", num)
	}

	// 版本快照应记录 source 与 ops 明细
	revs, err = repo.ListRevisions(ctx, userID, tableID, 10)
	if err != nil {
		t.Fatalf("读取版本失败: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("期望 2 个版本，实际 %d", len(revs))
	}
	latest := revs[0]
	if latest.Revision != 1 {
		t.Errorf("最新版本期望 1，实际 %d", latest.Revision)
	}
	if latest.ItemCount != 2 {
		t.Errorf("快照里的参数项数量期望 2，实际 %d", latest.ItemCount)
	}
	if latest.Source != SourceUser {
		t.Errorf("来源期望 user，实际 %q", latest.Source)
	}
}

func TestOptimisticLockRejectsStaleRevision(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "并发测试")

	created, err := svc.Create(ctx, userID, CreateRequest{Title: "并发表"})
	if err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tableID := created.Table.ID

	applyItems(t, svc, userID, tableID, "第一次修改",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "参数A", Value: "1"}},
	)

	// 模拟另一个客户端拿着过期的 base_revision 提交
	stale := 0
	_, err = svc.Apply(ctx, userID, tableID, ApplyRequest{
		BaseRevision: &stale,
		Ops: []ops.Op{{
			Op:   ops.OpAddItem,
			Item: &ops.Draft{Name: "参数B", Value: "2"},
		}},
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("期望 ErrRevisionConflict，实际 %v", err)
	}

	// 冲突不应产生任何写入
	detail, err := svc.Detail(ctx, userID, tableID)
	if err != nil {
		t.Fatalf("读取详情失败: %v", err)
	}
	if len(detail.Items) != 1 {
		t.Errorf("冲突后参数项不应变化，实际 %d 项", len(detail.Items))
	}
	if detail.Table.CurrentRevision != 1 {
		t.Errorf("冲突后版本号不应变化，实际 r%d", detail.Table.CurrentRevision)
	}
}

func TestApplyRejectsInvalidOps(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "校验测试")

	created, err := svc.Create(ctx, userID, CreateRequest{Title: "校验表"})
	if err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tableID := created.Table.ID

	applyItems(t, svc, userID, tableID, "建一项",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "重量", Value: "198g", ValueType: ops.TypeNumber}},
	)

	// 位置不存在的 item_id
	base := 1
	_, err = svc.Apply(ctx, userID, tableID, ApplyRequest{
		BaseRevision: &base,
		Ops:          []ops.Op{{Op: ops.OpUpdateItem, ItemID: "not-exist", Patch: &ops.Patch{Value: strPtr("1")}}},
	})
	if !errors.Is(err, ErrOpsRejected) {
		t.Fatalf("期望 ErrOpsRejected，实际 %v", err)
	}

	// 校验失败不应写入，版本号不动
	detail, err := svc.Detail(ctx, userID, tableID)
	if err != nil {
		t.Fatalf("读取详情失败: %v", err)
	}
	if detail.Table.CurrentRevision != 1 {
		t.Errorf("校验失败后版本号不应变化，实际 r%d", detail.Table.CurrentRevision)
	}
}

// ==================== 恢复与复制 ====================

func TestRestoreCreatesNewRevision(t *testing.T) {
	svc, repo, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "恢复测试")

	created, err := svc.Create(ctx, userID, CreateRequest{Title: "恢复表"})
	if err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	tableID := created.Table.ID

	applyItems(t, svc, userID, tableID, "第一版",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "参数A", Value: "1"}},
	)
	applyItems(t, svc, userID, tableID, "第二版",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "参数B", Value: "2"}},
	)

	detail, _ := svc.Detail(ctx, userID, tableID)
	if len(detail.Items) != 2 || detail.Table.CurrentRevision != 2 {
		t.Fatalf("前置状态不符: %d 项 / r%d", len(detail.Items), detail.Table.CurrentRevision)
	}

	// 恢复到 r1（当时只有参数A）
	res, err := svc.Restore(ctx, userID, tableID, 1)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	// 关键语义：恢复不是回退版本号，而是产生一个新版本
	if res.Revision != 3 {
		t.Errorf("恢复后应产生新版本 r3，实际 r%d", res.Revision)
	}

	detail, _ = svc.Detail(ctx, userID, tableID)
	if len(detail.Items) != 1 {
		t.Fatalf("恢复后应剩 1 项，实际 %d", len(detail.Items))
	}
	if detail.Items[0].Name != "参数A" {
		t.Errorf("恢复后应剩「参数A」，实际 %q", detail.Items[0].Name)
	}

	// 历史保持线性：r2 的快照仍可取回
	items, err := repo.GetRevisionItems(ctx, userID, tableID, 2)
	if err != nil {
		t.Fatalf("读取 r2 失败: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("r2 快照应仍为 2 项，实际 %d", len(items))
	}

	// 而且可以再恢复到 r2，等于重做
	if _, err := svc.Restore(ctx, userID, tableID, 2); err != nil {
		t.Fatalf("重做恢复失败: %v", err)
	}
	detail, _ = svc.Detail(ctx, userID, tableID)
	if len(detail.Items) != 2 {
		t.Errorf("重做后应回到 2 项，实际 %d", len(detail.Items))
	}
}

func TestRestoreToInitialState(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "清空测试")

	created, _ := svc.Create(ctx, userID, CreateRequest{Title: "清空表"})
	tableID := created.Table.ID

	applyItems(t, svc, userID, tableID, "加参数",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "参数A", Value: "1"}},
	)

	// 恢复到 r0 即回到建表时的空状态
	if _, err := svc.Restore(ctx, userID, tableID, 0); err != nil {
		t.Fatalf("恢复到 r0 失败: %v", err)
	}

	detail, _ := svc.Detail(ctx, userID, tableID)
	if len(detail.Items) != 0 {
		t.Errorf("恢复到 r0 后应为空表，实际 %d 项", len(detail.Items))
	}
}

func TestDuplicateTable(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "复制测试")

	created, _ := svc.Create(ctx, userID, CreateRequest{Title: "原始产品"})
	srcID := created.Table.ID

	applyItems(t, svc, userID, srcID, "加参数",
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "电池容量", Value: "5000mAh", ValueType: ops.TypeNumber, GroupName: "续航"}},
		ops.Op{Op: ops.OpAddItem, Item: &ops.Draft{Name: "重量", Value: "198g", ValueType: ops.TypeNumber, GroupName: "外观"}},
	)

	copied, err := svc.Duplicate(ctx, userID, srcID, DuplicateRequest{Title: "新产品"})
	if err != nil {
		t.Fatalf("复制失败: %v", err)
	}

	if copied.Table.ID == srcID {
		t.Fatal("副本应是新表")
	}
	if copied.Table.Title != "新产品" {
		t.Errorf("副本标题错误: %q", copied.Table.Title)
	}
	if len(copied.Items) != 2 {
		t.Fatalf("副本应有 2 项，实际 %d", len(copied.Items))
	}

	// 副本从 r0 开始，不继承源表历史
	if copied.Table.CurrentRevision != 0 {
		t.Errorf("副本版本号应从 0 开始，实际 r%d", copied.Table.CurrentRevision)
	}

	// 参数项 id 必须重新生成，否则两张表共享主键
	_, srcItems, _ := svc.repo.GetTableWithItems(ctx, userID, srcID)
	srcIDs := map[string]bool{}
	for _, it := range srcItems {
		srcIDs[it.ID] = true
	}
	for _, it := range copied.Items {
		if srcIDs[it.ID] {
			t.Errorf("副本参数项复用了源表 id: %s", it.ID)
		}
	}

	// 改副本不应影响源表
	copiedBase := copied.Table.CurrentRevision
	if _, err := svc.Apply(ctx, userID, copied.Table.ID, ApplyRequest{
		BaseRevision: &copiedBase,
		Ops: []ops.Op{{
			Op:     ops.OpUpdateItem,
			ItemID: copied.Items[0].ID,
			Patch:  &ops.Patch{Value: strPtr("9999")},
		}},
	}); err != nil {
		t.Fatalf("改副本失败: %v", err)
	}

	_, srcItems, _ = svc.repo.GetTableWithItems(ctx, userID, srcID)
	for _, it := range srcItems {
		if it.Name == "电池容量" && it.Value == "9999" {
			t.Error("改副本影响了源表")
		}
	}
}

// ==================== 其它 ====================

func TestSoftDeleteHidesTable(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userID := createTestUser(t, db, "删除测试")

	created, _ := svc.Create(ctx, userID, CreateRequest{Title: "待删除"})
	tableID := created.Table.ID

	if err := svc.Delete(ctx, userID, tableID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	if _, err := svc.Detail(ctx, userID, tableID); !errors.Is(err, ErrTableNotFound) {
		t.Errorf("软删除后应查不到，实际 %v", err)
	}

	list, _ := svc.List(ctx, userID, "", 1, 100)
	for _, item := range list.Items {
		if item.ID == tableID {
			t.Error("软删除的表不应出现在列表里")
		}
	}
}

func TestListScopedToUser(t *testing.T) {
	svc, _, db := newTestService(t)
	ctx := context.Background()
	userA := createTestUser(t, db, "用户A")
	userB := createTestUser(t, db, "用户B")

	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, userA, CreateRequest{Title: "A的表"}); err != nil {
			t.Fatalf("建表失败: %v", err)
		}
	}
	if _, err := svc.Create(ctx, userB, CreateRequest{Title: "B的表"}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	listA, err := svc.List(ctx, userA, "", 1, 100)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if listA.Total != 3 {
		t.Errorf("A 应看到 3 张表，实际 %d", listA.Total)
	}
	for _, item := range listA.Items {
		if item.Title != "A的表" {
			t.Errorf("A 的列表里出现了别人的表: %q", item.Title)
		}
	}

	listB, _ := svc.List(ctx, userB, "", 1, 100)
	if listB.Total != 1 {
		t.Errorf("B 应看到 1 张表，实际 %d", listB.Total)
	}
}

func TestCreateRejectsBlankTitle(t *testing.T) {
	svc, _, db := newTestService(t)
	userID := createTestUser(t, db, "空标题")

	if _, err := svc.Create(context.Background(), userID, CreateRequest{Title: "   "}); !errors.Is(err, ErrTitleRequired) {
		t.Errorf("期望 ErrTitleRequired，实际 %v", err)
	}
}

func strPtr(s string) *string { return &s }
