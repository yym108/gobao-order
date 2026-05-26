// Package integration 提供 Order 服务对外部依赖的接线适配实现。
// 本文件负责把订单领域对象发布为 NATS 事件，供后续 Payment 和补偿链路订阅。
package integration

import (
	"context"
	"encoding/json"

	"github.com/yym108/gobao-order/internal/domain"
	"github.com/yym108/gobao-pkg/mq"
)

type orderCreatedPayload struct {
	ID          int64  `json:"id"`           // 真实订单 ID
	OrderNo     string `json:"order_no"`     // 订单号
	UserID      int64  `json:"user_id"`      // 用户 ID
	TotalAmount int64  `json:"total_amount"` // 订单总金额
	Status      string `json:"status"`       // 当前订单状态
}

// OrderEventPublisher 基于 NATS JetStream 实现订单事件发布器。
type OrderEventPublisher struct {
	bus              *mq.Bus // 底层消息总线
	createdSubject   string  // 订单创建事件主题
	cancelledSubject string  // 订单取消事件主题
}

// NewOrderEventPublisher 创建订单事件发布器。
func NewOrderEventPublisher(bus *mq.Bus, createdSubject, cancelledSubject string) *OrderEventPublisher {
	return &OrderEventPublisher{
		bus:              bus,
		createdSubject:   createdSubject,
		cancelledSubject: cancelledSubject,
	}
}

// PublishOrderCreated 发布订单创建完成事件。
func (p *OrderEventPublisher) PublishOrderCreated(ctx context.Context, order *domain.Order) error {
	return p.publish(ctx, p.createdSubject, order)
}

// PublishOrderCancelled 发布订单取消事件。
func (p *OrderEventPublisher) PublishOrderCancelled(ctx context.Context, order *domain.Order) error {
	return p.publish(ctx, p.cancelledSubject, order)
}

// publish 统一把订单聚合序列化后投递到指定主题。
func (p *OrderEventPublisher) publish(ctx context.Context, subject string, order *domain.Order) error {
	if p == nil || p.bus == nil || order == nil || subject == "" {
		return nil
	}
	payload, err := json.Marshal(orderCreatedPayload{
		ID:          order.ID,
		OrderNo:     order.OrderNo,
		UserID:      order.UserID,
		TotalAmount: order.TotalAmount,
		Status:      order.Status,
	})
	if err != nil {
		return err
	}
	return p.bus.Publish(ctx, subject, payload)
}
