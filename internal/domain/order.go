// Package domain 定义 Order 服务的核心领域对象与常量。
// 该层不依赖任何外部框架，负责表达订单聚合自身的业务结构。
package domain

import "time"

// Order 表示订单聚合根。
// 当前最小实现先覆盖下单主数据与订单明细，后续可继续扩展支付、收货和状态流转字段。
type Order struct {
	ID            int64       // 订单主键
	OrderNo       string      // 业务订单号
	UserID        int64       // 下单用户 ID
	RequestID     string      // 幂等请求 ID
	Status        string      // 订单状态
	TotalAmount   int64       // 订单总金额，单位为分
	TotalQuantity int32       // 商品总件数
	ReceiverName  string      // 收货人姓名快照
	ReceiverPhone string      // 收货手机号快照
	Province      string      // 省份快照
	City          string      // 城市快照
	District      string      // 区县快照
	AddressLine   string      // 详细地址快照
	PostalCode    string      // 邮编快照
	CreatedAt     time.Time   // 创建时间
	UpdatedAt     time.Time   // 更新时间
	Items         []OrderItem // 订单明细
}

// OrderItem 表示订单中的单个售卖单元快照。
// 这里直接固化商品名称、图片和规格摘要，避免后续商品变更影响历史订单展示。
type OrderItem struct {
	ID            int64  // 明细主键
	OrderID       int64  // 所属订单 ID
	ProductID     int64  // 商品 ID
	SKUID         int64  // SKU ID
	SKUCode       string // SKU 编码快照
	SKUTitle      string // SKU 标题快照
	Name          string // 商品名称快照
	ImageURL      string // 商品图片快照
	OptionSummary string // SKU 规格摘要快照
	Price         int64  // 下单单价，单位为分
	Quantity      int32  // 购买数量
	Amount        int64  // 该行小计金额，单位为分
}

const (
	// OrderStatusCreated 表示订单已创建，后续等待支付或进一步流转。
	OrderStatusCreated = "CREATED"
	// OrderStatusPaid 表示订单已支付，支付成功事件已被订单服务消费。
	OrderStatusPaid = "PAID"
	// OrderStatusPaymentFailed 表示订单支付失败，当前用于 mock 支付失败后的终态。
	OrderStatusPaymentFailed = "PAYMENT_FAILED"
	// OrderStatusCancelled 表示订单已取消，当前最小实现用于未支付订单主动取消场景。
	OrderStatusCancelled = "CANCELLED"
)
