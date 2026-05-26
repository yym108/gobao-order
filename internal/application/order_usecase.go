// Package application 提供 Order 服务的业务编排层。
// 该层负责把订单仓储、商品读取、库存协作和幂等控制组织成稳定的用例。
package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/yym108/gobao-order/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
)

const (
	// defaultIdempotencyTTL 定义普通下单请求的默认幂等窗口。
	defaultIdempotencyTTL = 10 * time.Minute
)

// ProductGateway 抽象订单对 Product 服务的依赖。
// 当前最小实现只依赖 SKU 真值读取和商品级库存扣减，后续再演进为更细粒度的 SKU 库存协作。
type ProductGateway interface {
	// GetSKUByID 按 sku_id 查询当前可售 SKU 快照，未找到返回 (nil, nil)。
	GetSKUByID(ctx context.Context, skuID int64) (*SKUView, error)
	// DeductStock 按商品维度扣减库存，当前先复用 Product 现有内部能力。
	DeductStock(ctx context.Context, productID int64, quantity int32) error
	// RestoreStock 按商品维度回补库存，为取消订单预留接线点。
	RestoreStock(ctx context.Context, productID int64, quantity int32) error
}

// IdempotencyStore 抽象幂等占用能力。
// Order 应用层只关心“是否已占用”，具体可由 Redis Guard 等实现提供。
type IdempotencyStore interface {
	// Acquire 尝试占用幂等键，true 表示首次请求，false 表示重复请求。
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// OrderEventPublisher 抽象订单事件发布能力。
// 当前 I3 只要求在创建与取消后发布最小事件，为 Payment 和补偿链路留出边界。
type OrderEventPublisher interface {
	// PublishOrderCreated 发布订单创建完成事件。
	PublishOrderCreated(ctx context.Context, order *domain.Order) error
	// PublishOrderCancelled 发布订单取消事件。
	PublishOrderCancelled(ctx context.Context, order *domain.Order) error
}

// SKUView 表示订单应用层需要的最小 SKU 真值快照。
// 价格、标题和规格摘要都应以后端商品服务返回为准，前端不参与计算。
type SKUView struct {
	ProductID      int64   // 所属商品 ID
	SKUID          int64   // SKU ID
	SKUCode        string  // SKU 编码
	Title          string  // SKU 标题
	ProductName    string  // 商品名称
	ImageURL       string  // 商品图片
	OptionSummary  string  // 规格摘要
	OptionValueIDs []int64 // 规格值 ID 列表，后续扩展校验时使用
	Price          int64   // SKU 单价，单位为分
	StockQuantity  int32   // 当前库存，仅作应用层前置判断参考
	Status         int32   // SKU 状态，1=可售
}

// CreateOrderCommand 描述创建订单时所需的最小输入。
// 当前阶段先支持单 SKU 下单，后续再扩展为多明细订单。
type CreateOrderCommand struct {
	UserID        int64  // 下单用户 ID
	RequestID     string // 幂等请求 ID
	SKUID         int64  // 目标 SKU ID
	Quantity      int32  // 购买数量
	ReceiverName  string // 收货人姓名
	ReceiverPhone string // 收货手机号
	Province      string // 省份
	City          string // 城市
	District      string // 区县
	AddressLine   string // 详细地址
	PostalCode    string // 邮编
}

// OrderUseCase 负责普通下单场景的业务编排。
type OrderUseCase struct {
	orderRepo   domain.OrderRepository // 订单仓储
	productGW   ProductGateway         // 商品服务协作接口
	idemStore   IdempotencyStore       // 幂等存储
	eventPub    OrderEventPublisher    // 订单事件发布器
	idemTTL     time.Duration          // 幂等窗口
	timeNowFunc func() time.Time       // 当前时间函数，便于测试替换
}

// NewOrderUseCase 构造订单应用层实例。
func NewOrderUseCase(orderRepo domain.OrderRepository, productGW ProductGateway, idemStore IdempotencyStore, eventPub OrderEventPublisher) *OrderUseCase {
	return &OrderUseCase{
		orderRepo:   orderRepo,
		productGW:   productGW,
		idemStore:   idemStore,
		eventPub:    eventPub,
		idemTTL:     defaultIdempotencyTTL,
		timeNowFunc: time.Now,
	}
}

// CreateOrder 基于后端 SKU 真值创建订单。
// 业务规则：
// 1. user_id、request_id、sku_id、quantity 必须有效；
// 2. 幂等键按 user_id + request_id 生成；
// 3. SKU 快照、价格和规格摘要只认 Product 返回值；
// 4. 当前库存扣减先走商品级接口，避免与现有 Product 实现脱节。
func (uc *OrderUseCase) CreateOrder(ctx context.Context, cmd CreateOrderCommand) (*domain.Order, error) {
	if cmd.UserID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
	if cmd.RequestID == "" {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "request_id 不能为空")
	}
	if cmd.SKUID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "sku_id 无效")
	}
	if cmd.Quantity <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "quantity 必须大于 0")
	}

	acquired, err := uc.idemStore.Acquire(ctx, buildIdempotencyKey(cmd.UserID, cmd.RequestID), uc.idemTTL)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, pkgerrors.New(pkgerrors.CodeConflict, "重复下单请求")
	}

	sku, err := uc.productGW.GetSKUByID(ctx, cmd.SKUID)
	if err != nil {
		return nil, err
	}
	if sku == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "SKU 不存在")
	}
	if sku.Status != 1 {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "SKU 当前不可售")
	}
	if sku.StockQuantity <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "SKU 当前缺货")
	}

	if err := uc.productGW.DeductStock(ctx, sku.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	now := uc.timeNowFunc()
	order := &domain.Order{
		OrderNo:       buildOrderNo(now, cmd.UserID),
		UserID:        cmd.UserID,
		RequestID:     cmd.RequestID,
		Status:        domain.OrderStatusCreated,
		TotalAmount:   sku.Price * int64(cmd.Quantity),
		TotalQuantity: cmd.Quantity,
		ReceiverName:  strings.TrimSpace(cmd.ReceiverName),
		ReceiverPhone: strings.TrimSpace(cmd.ReceiverPhone),
		Province:      strings.TrimSpace(cmd.Province),
		City:          strings.TrimSpace(cmd.City),
		District:      strings.TrimSpace(cmd.District),
		AddressLine:   strings.TrimSpace(cmd.AddressLine),
		PostalCode:    strings.TrimSpace(cmd.PostalCode),
		CreatedAt:     now,
		UpdatedAt:     now,
		Items: []domain.OrderItem{
			{
				ProductID:     sku.ProductID,
				SKUID:         sku.SKUID,
				SKUCode:       sku.SKUCode,
				SKUTitle:      sku.Title,
				Name:          sku.ProductName,
				ImageURL:      sku.ImageURL,
				OptionSummary: sku.OptionSummary,
				Price:         sku.Price,
				Quantity:      cmd.Quantity,
				Amount:        sku.Price * int64(cmd.Quantity),
			},
		},
	}
	if err := uc.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}
	if uc.eventPub != nil {
		if err := uc.eventPub.PublishOrderCreated(ctx, order); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// GetOrderByID 按订单 ID 查询订单聚合，并校验该订单是否属于当前用户。
func (uc *OrderUseCase) GetOrderByID(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	if userID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "订单不存在")
	}
	if order.UserID != userID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "无权访问该订单")
	}
	return order, nil
}

// ListOrdersByUserID 按用户分页查询订单列表。
func (uc *OrderUseCase) ListOrdersByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error) {
	if userID <= 0 {
		return nil, 0, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return uc.orderRepo.ListByUserID(ctx, userID, page, pageSize)
}

// CancelOrder 取消当前用户的未支付订单，并回补关联商品库存。
// 当前最小实现仅允许取消 CREATED 状态订单，不涉及支付补偿与事件投递。
func (uc *OrderUseCase) CancelOrder(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	if userID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 无效")
	}
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}

	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "订单不存在")
	}
	if order.UserID != userID {
		return nil, pkgerrors.New(pkgerrors.CodeForbidden, "无权操作该订单")
	}
	if order.Status != domain.OrderStatusCreated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可取消")
	}

	for _, item := range order.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			continue
		}
		if err := uc.productGW.RestoreStock(ctx, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}
	}

	now := uc.timeNowFunc()
	updated, err := uc.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusCancelled, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可取消")
	}

	order.Status = domain.OrderStatusCancelled
	order.UpdatedAt = now
	if uc.eventPub != nil {
		if err := uc.eventPub.PublishOrderCancelled(ctx, order); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// MarkOrderPaid 处理支付成功后的订单状态推进。
// 当前仅允许把 CREATED 状态推进到 PAID，避免覆盖已取消等终态订单。
func (uc *OrderUseCase) MarkOrderPaid(ctx context.Context, orderID int64) (*domain.Order, error) {
	return uc.markOrderStatus(ctx, orderID, domain.OrderStatusPaid)
}

// MarkOrderPaymentFailed 处理支付失败后的订单状态推进。
// 当前策略为“支付失败即取消订单”，并同步回补库存；若未来支持重新支付，再单独扩展状态机。
func (uc *OrderUseCase) MarkOrderPaymentFailed(ctx context.Context, orderID int64) (*domain.Order, error) {
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}

	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "订单不存在")
	}
	if order.Status != domain.OrderStatusCreated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可推进支付结果")
	}

	for _, item := range order.Items {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			continue
		}
		if err := uc.productGW.RestoreStock(ctx, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}
	}

	now := uc.timeNowFunc()
	updated, err := uc.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, domain.OrderStatusCancelled, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可推进支付结果")
	}

	order.Status = domain.OrderStatusCancelled
	order.UpdatedAt = now
	if uc.eventPub != nil {
		if err := uc.eventPub.PublishOrderCancelled(ctx, order); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// markOrderStatus 统一处理支付事件驱动的订单状态推进。
func (uc *OrderUseCase) markOrderStatus(ctx context.Context, orderID int64, targetStatus string) (*domain.Order, error) {
	if orderID <= 0 {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 无效")
	}

	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, pkgerrors.New(pkgerrors.CodeNotFound, "订单不存在")
	}
	if order.Status != domain.OrderStatusCreated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可推进支付结果")
	}

	now := uc.timeNowFunc()
	updated, err := uc.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCreated, targetStatus, now)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可推进支付结果")
	}
	order.Status = targetStatus
	order.UpdatedAt = now
	return order, nil
}

// buildIdempotencyKey 统一生成订单创建幂等键。
func buildIdempotencyKey(userID int64, requestID string) string {
	return strconv.FormatInt(userID, 10) + ":" + strings.TrimSpace(requestID)
}

// buildOrderNo 生成最小可用业务订单号。
// 当前只要求在服务内唯一且便于日志追踪，后续可替换为独立号段服务。
func buildOrderNo(now time.Time, userID int64) string {
	return "ORD-" + now.UTC().Format("20060102150405.000000000") + "-" + strconv.FormatInt(userID, 10)
}
