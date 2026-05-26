package domain

import (
	"context"
	"time"
)

// OrderRepository 定义订单聚合的仓储抽象。
// application 层只依赖该接口，方便后续替换持久化实现或补充事务编排。
type OrderRepository interface {
	// Create 持久化订单聚合，并回填订单与明细主键。
	Create(ctx context.Context, order *Order) error
	// FindByID 按主键查询订单聚合，未找到时返回 (nil, nil)。
	FindByID(ctx context.Context, id int64) (*Order, error)
	// ListByUserID 按用户分页查询订单列表，返回当前页订单和总数。
	ListByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*Order, int64, error)
	// UpdateStatus 在旧状态匹配时原子更新订单状态，返回是否成功更新。
	UpdateStatus(ctx context.Context, id int64, fromStatus, toStatus string, updatedAt time.Time) (bool, error)
}
