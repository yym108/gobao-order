//go:build integration

package mysql_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	mysqlrepo "github.com/yym108/gobao-order/internal/adapter/repository/mysql"
	"github.com/yym108/gobao-order/internal/domain"
)

// setupOrderRepo 创建 SQLite 内存数据库并完成订单相关表迁移。
func setupOrderRepo(t *testing.T) (*mysqlrepo.OrderRepo, func()) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&mysqlrepo.OrderModel{}, &mysqlrepo.OrderItemModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return mysqlrepo.NewOrderRepo(db), func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	}
}

// TestOrderRepo_CreateAndFindByID 验证最小订单聚合可以写入并按主键查回。
func TestOrderRepo_CreateAndFindByID(t *testing.T) {
	repo, cleanup := setupOrderRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	order := &domain.Order{
		OrderNo:       "ORD-20260518-001",
		UserID:        1001,
		RequestID:     "req-001",
		Status:        domain.OrderStatusCreated,
		TotalAmount:   1999800,
		TotalQuantity: 2,
		ReceiverName:  "张三",
		ReceiverPhone: "13800138000",
		Province:      "上海市",
		City:          "上海市",
		District:      "浦东新区",
		AddressLine:   "世纪大道 100 号 18 层",
		PostalCode:    "200120",
		CreatedAt:     now,
		UpdatedAt:     now,
		Items: []domain.OrderItem{
			{
				ProductID:     1001,
				SKUID:         1001002,
				SKUCode:       "MBA-M4-16G-512G",
				SKUTitle:      "MacBook Air 13 英寸 M4 / 16GB / 512GB",
				Name:          "MacBook Air",
				ImageURL:      "https://example.com/mac.png",
				OptionSummary: "M4 / 16GB / 512GB",
				Price:         999900,
				Quantity:      2,
				Amount:        1999800,
			},
		},
	}

	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("create: %v", err)
	}
	if order.ID == 0 {
		t.Fatal("expect order id assigned")
	}
	if len(order.Items) != 1 || order.Items[0].ID == 0 {
		t.Fatal("expect order item id assigned")
	}

	got, err := repo.FindByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got == nil {
		t.Fatal("expect order found")
	}
	if got.OrderNo != order.OrderNo {
		t.Fatalf("order no want %q, got %q", order.OrderNo, got.OrderNo)
	}
	if got.UserID != order.UserID {
		t.Fatalf("user id want %d, got %d", order.UserID, got.UserID)
	}
	if got.TotalAmount != order.TotalAmount {
		t.Fatalf("total amount want %d, got %d", order.TotalAmount, got.TotalAmount)
	}
	if got.ReceiverName != order.ReceiverName || got.AddressLine != order.AddressLine {
		t.Fatalf("address snapshot not persisted, got receiver=%q address=%q", got.ReceiverName, got.AddressLine)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items len want 1, got %d", len(got.Items))
	}
	if got.Items[0].SKUID != order.Items[0].SKUID {
		t.Fatalf("sku id want %d, got %d", order.Items[0].SKUID, got.Items[0].SKUID)
	}
	if got.Items[0].SKUCode != order.Items[0].SKUCode || got.Items[0].SKUTitle != order.Items[0].SKUTitle {
		t.Fatalf("sku snapshot not persisted, got code=%q title=%q", got.Items[0].SKUCode, got.Items[0].SKUTitle)
	}
	if got.Items[0].Amount != order.Items[0].Amount {
		t.Fatalf("item amount want %d, got %d", order.Items[0].Amount, got.Items[0].Amount)
	}
}

// TestOrderRepo_ListByUserID 验证仓储层可以按用户分页返回订单列表。
func TestOrderRepo_ListByUserID(t *testing.T) {
	repo, cleanup := setupOrderRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 5, 18, 11, 0, 0, 0, time.UTC)
	for _, orderNo := range []string{"ORD-001", "ORD-002", "ORD-003"} {
		order := &domain.Order{
			OrderNo:       orderNo,
			UserID:        2001,
			RequestID:     "req-" + orderNo,
			Status:        domain.OrderStatusCreated,
			TotalAmount:   999900,
			TotalQuantity: 1,
			CreatedAt:     now,
			UpdatedAt:     now,
			Items: []domain.OrderItem{
				{
					ProductID: 1001,
					SKUID:     1001002,
					Price:     999900,
					Quantity:  1,
					Amount:    999900,
				},
			},
		}
		if err := repo.Create(ctx, order); err != nil {
			t.Fatalf("create %s: %v", orderNo, err)
		}
	}

	items, total, err := repo.ListByUserID(ctx, 2001, 1, 2)
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if total != 3 {
		t.Fatalf("total want 3, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("items len want 2, got %d", len(items))
	}
	if items[0].OrderNo != "ORD-003" || items[1].OrderNo != "ORD-002" {
		t.Fatalf("unexpected order sequence: %q, %q", items[0].OrderNo, items[1].OrderNo)
	}
}

// TestOrderRepo_UpdateStatus 验证仓储层仅在旧状态匹配时才会更新订单状态。
func TestOrderRepo_UpdateStatus(t *testing.T) {
	repo, cleanup := setupOrderRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	order := &domain.Order{
		OrderNo:       "ORD-CANCEL-001",
		UserID:        3001,
		RequestID:     "req-cancel-001",
		Status:        domain.OrderStatusCreated,
		TotalAmount:   999900,
		TotalQuantity: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
		Items: []domain.OrderItem{
			{ProductID: 1001, SKUID: 1001002, Price: 999900, Quantity: 1, Amount: 999900},
		},
	}
	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusCancelled, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if !updated {
		t.Fatal("expect status updated")
	}

	got, err := repo.FindByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got == nil || got.Status != domain.OrderStatusCancelled {
		t.Fatalf("unexpected status after update: %+v", got)
	}

	updated, err = repo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusCancelled, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("update status with stale from status: %v", err)
	}
	if updated {
		t.Fatal("expect stale status update rejected")
	}
}

// TestOrderRepo_UpdateStatus_ToPaidAndFailed 验证订单状态可以从 CREATED 分别推进到 PAID 和 PAYMENT_FAILED。
func TestOrderRepo_UpdateStatus_ToPaidAndFailed(t *testing.T) {
	repo, cleanup := setupOrderRepo(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 5, 18, 13, 0, 0, 0, time.UTC)
	order := &domain.Order{
		OrderNo:       "ORD-PAY-001",
		UserID:        4001,
		RequestID:     "req-pay-001",
		Status:        domain.OrderStatusCreated,
		TotalAmount:   999900,
		TotalQuantity: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
		Items: []domain.OrderItem{
			{ProductID: 1001, SKUID: 1001002, Price: 999900, Quantity: 1, Amount: 999900},
		},
	}
	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusPaid, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update status to paid: %v", err)
	}
	if !updated {
		t.Fatal("expect status updated to paid")
	}

	got, err := repo.FindByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got == nil || got.Status != domain.OrderStatusPaid {
		t.Fatalf("unexpected paid status after update: %+v", got)
	}

	other, err := repo.UpdateStatus(ctx, order.ID, domain.OrderStatusPaid, domain.OrderStatusPaymentFailed, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("update status to failed: %v", err)
	}
	if !other {
		t.Fatal("expect status updated to payment failed")
	}

	got, err = repo.FindByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("find by id after failed: %v", err)
	}
	if got == nil || got.Status != domain.OrderStatusPaymentFailed {
		t.Fatalf("unexpected failed status after update: %+v", got)
	}
}
