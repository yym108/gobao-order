// Package mysql 提供 Order 服务基于 GORM 的 MySQL 仓储实现。
// 生产环境连接 MySQL，集成测试使用 SQLite 内存库验证聚合持久化行为。
package mysql

import (
	"time"

	"github.com/yym108/gobao-order/internal/domain"
)

// OrderModel 是订单主表的 GORM 模型。
type OrderModel struct {
	ID            int64            `gorm:"column:id;primaryKey;autoIncrement"`                         // 订单主键
	OrderNo       string           `gorm:"column:order_no;type:varchar(64);not null;uniqueIndex"`      // 业务订单号
	UserID        int64            `gorm:"column:user_id;not null;index"`                              // 下单用户 ID
	RequestID     string           `gorm:"column:request_id;type:varchar(64);not null;index"`          // 幂等请求 ID
	Status        string           `gorm:"column:status;type:varchar(32);not null;index"`              // 订单状态
	TotalAmount   int64            `gorm:"column:total_amount;not null"`                               // 订单总金额
	TotalQuantity int32            `gorm:"column:total_quantity;not null"`                             // 商品总件数
	ReceiverName  string           `gorm:"column:receiver_name;type:varchar(64);not null;default:''"`  // 收货人姓名快照
	ReceiverPhone string           `gorm:"column:receiver_phone;type:varchar(32);not null;default:''"` // 收货手机号快照
	Province      string           `gorm:"column:province;type:varchar(64);not null;default:''"`       // 省份快照
	City          string           `gorm:"column:city;type:varchar(64);not null;default:''"`           // 城市快照
	District      string           `gorm:"column:district;type:varchar(64);not null;default:''"`       // 区县快照
	AddressLine   string           `gorm:"column:address_line;type:varchar(255);not null;default:''"`  // 详细地址快照
	PostalCode    string           `gorm:"column:postal_code;type:varchar(32);not null;default:''"`    // 邮编快照
	CreatedAt     time.Time        `gorm:"column:created_at"`                                          // 创建时间
	UpdatedAt     time.Time        `gorm:"column:updated_at"`                                          // 更新时间
	Items         []OrderItemModel `gorm:"foreignKey:OrderID;references:ID"`                           // 关联明细
}

// TableName 指定订单主表名。
func (OrderModel) TableName() string { return "orders" }

// OrderItemModel 是订单明细表的 GORM 模型。
type OrderItemModel struct {
	ID            int64     `gorm:"column:id;primaryKey;autoIncrement"`                     // 明细主键
	OrderID       int64     `gorm:"column:order_id;not null;index"`                         // 所属订单 ID
	ProductID     int64     `gorm:"column:product_id;not null;index"`                       // 商品 ID
	SKUID         int64     `gorm:"column:sku_id;not null;index"`                           // SKU ID
	SKUCode       string    `gorm:"column:sku_code;type:varchar(100);not null;default:''"`  // SKU 编码快照
	SKUTitle      string    `gorm:"column:sku_title;type:varchar(200);not null;default:''"` // SKU 标题快照
	Name          string    `gorm:"column:name;type:varchar(200);not null"`                 // 商品名称快照
	ImageURL      string    `gorm:"column:image_url;type:varchar(500)"`                     // 商品图片快照
	OptionSummary string    `gorm:"column:option_summary;type:varchar(255)"`                // SKU 规格摘要
	Price         int64     `gorm:"column:price;not null"`                                  // 下单单价
	Quantity      int32     `gorm:"column:quantity;not null"`                               // 购买数量
	Amount        int64     `gorm:"column:amount;not null"`                                 // 明细小计金额
	CreatedAt     time.Time `gorm:"column:created_at"`                                      // 创建时间
	UpdatedAt     time.Time `gorm:"column:updated_at"`                                      // 更新时间
}

// TableName 指定订单明细表名。
func (OrderItemModel) TableName() string { return "order_items" }

// orderToModel 将领域订单聚合转换为 GORM 模型。
func orderToModel(order *domain.Order) *OrderModel {
	if order == nil {
		return nil
	}
	items := make([]OrderItemModel, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, OrderItemModel{
			ID:            item.ID,
			OrderID:       item.OrderID,
			ProductID:     item.ProductID,
			SKUID:         item.SKUID,
			SKUCode:       item.SKUCode,
			SKUTitle:      item.SKUTitle,
			Name:          item.Name,
			ImageURL:      item.ImageURL,
			OptionSummary: item.OptionSummary,
			Price:         item.Price,
			Quantity:      item.Quantity,
			Amount:        item.Amount,
		})
	}
	return &OrderModel{
		ID:            order.ID,
		OrderNo:       order.OrderNo,
		UserID:        order.UserID,
		RequestID:     order.RequestID,
		Status:        order.Status,
		TotalAmount:   order.TotalAmount,
		TotalQuantity: order.TotalQuantity,
		ReceiverName:  order.ReceiverName,
		ReceiverPhone: order.ReceiverPhone,
		Province:      order.Province,
		City:          order.City,
		District:      order.District,
		AddressLine:   order.AddressLine,
		PostalCode:    order.PostalCode,
		CreatedAt:     order.CreatedAt,
		UpdatedAt:     order.UpdatedAt,
		Items:         items,
	}
}

// orderToDomain 将 GORM 订单聚合转换为领域对象。
func orderToDomain(model *OrderModel) *domain.Order {
	if model == nil {
		return nil
	}
	items := make([]domain.OrderItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, domain.OrderItem{
			ID:            item.ID,
			OrderID:       item.OrderID,
			ProductID:     item.ProductID,
			SKUID:         item.SKUID,
			SKUCode:       item.SKUCode,
			SKUTitle:      item.SKUTitle,
			Name:          item.Name,
			ImageURL:      item.ImageURL,
			OptionSummary: item.OptionSummary,
			Price:         item.Price,
			Quantity:      item.Quantity,
			Amount:        item.Amount,
		})
	}
	return &domain.Order{
		ID:            model.ID,
		OrderNo:       model.OrderNo,
		UserID:        model.UserID,
		RequestID:     model.RequestID,
		Status:        model.Status,
		TotalAmount:   model.TotalAmount,
		TotalQuantity: model.TotalQuantity,
		ReceiverName:  model.ReceiverName,
		ReceiverPhone: model.ReceiverPhone,
		Province:      model.Province,
		City:          model.City,
		District:      model.District,
		AddressLine:   model.AddressLine,
		PostalCode:    model.PostalCode,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		Items:         items,
	}
}
