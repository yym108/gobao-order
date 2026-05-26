package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/yym108/gobao-order/internal/domain"
)

// OrderRepo 是订单聚合的 GORM 仓储实现。
type OrderRepo struct {
	db *gorm.DB // 底层数据库连接
}

// NewOrderRepo 创建订单仓储实例。
func NewOrderRepo(db *gorm.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

// Create 在单事务内持久化订单主表与明细表，并回填领域对象主键。
func (r *OrderRepo) Create(ctx context.Context, order *domain.Order) error {
	model := orderToModel(order)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(model).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	order.ID = model.ID
	order.CreatedAt = model.CreatedAt
	order.UpdatedAt = model.UpdatedAt
	for idx := range model.Items {
		order.Items[idx].ID = model.Items[idx].ID
		order.Items[idx].OrderID = model.Items[idx].OrderID
	}
	return nil
}

// FindByID 按主键查询完整订单聚合，未找到时返回 nil。
func (r *OrderRepo) FindByID(ctx context.Context, id int64) (*domain.Order, error) {
	var model OrderModel
	err := r.db.WithContext(ctx).Preload("Items").First(&model, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return orderToDomain(&model), nil
}

// ListByUserID 按用户分页查询订单列表，按 id 倒序返回最新订单。
func (r *OrderRepo) ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&OrderModel{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []OrderModel
	if err := query.Preload("Items").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&models).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*domain.Order, 0, len(models))
	for idx := range models {
		items = append(items, orderToDomain(&models[idx]))
	}
	return items, total, nil
}

// UpdateStatus 在旧状态匹配时原子更新订单状态，避免重复取消或并发覆盖。
func (r *OrderRepo) UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string, updatedAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&OrderModel{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
