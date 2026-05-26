// Package grpc 提供 Order 服务的 gRPC Handler 实现。
// 职责: 入参校验 → 调用订单用例 → 领域对象转 proto 响应 → 错误码映射 gRPC status。
package grpc

import (
	"context"

	"github.com/yym108/gobao-order/internal/application"
	"github.com/yym108/gobao-order/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
)

// orderUseCase 抽象订单应用层能力，便于 handler 测试时替换。
type orderUseCase interface {
	// CreateOrder 创建单 SKU 订单，并返回完整订单聚合。
	CreateOrder(ctx context.Context, cmd application.CreateOrderCommand) (*domain.Order, error)
	// GetOrderByID 按订单 ID 查询当前用户的订单。
	GetOrderByID(ctx context.Context, userID, orderID int64) (*domain.Order, error)
	// ListOrdersByUserID 按用户分页查询订单列表。
	ListOrdersByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error)
	// CancelOrder 取消当前用户的订单。
	CancelOrder(ctx context.Context, userID, orderID int64) (*domain.Order, error)
}

// OrderHandler 实现 proto 生成的 OrderServiceServer 接口。
type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer              // 向前兼容嵌入
	orderUC                                 orderUseCase // 订单应用层用例
}

// NewOrderHandler 构造订单 gRPC Handler。
func NewOrderHandler(orderUC orderUseCase) *OrderHandler {
	return &OrderHandler{orderUC: orderUC}
}

// CreateOrder 创建订单 RPC。
// 当前阶段只开放单 SKU 下单，请求中的价格与规格不接受前端传入，全部以后端 SKU 真值为准。
func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetRequestId() == "" {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "request_id 不能为空")).Err()
	}
	if req.GetSkuId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "sku_id 必须为正数")).Err()
	}
	if req.GetQuantity() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "quantity 必须大于 0")).Err()
	}

	order, err := h.orderUC.CreateOrder(ctx, application.CreateOrderCommand{
		UserID:        req.GetUserId(),
		RequestID:     req.GetRequestId(),
		SKUID:         req.GetSkuId(),
		Quantity:      req.GetQuantity(),
		ReceiverName:  req.GetReceiverName(),
		ReceiverPhone: req.GetReceiverPhone(),
		Province:      req.GetProvince(),
		City:          req.GetCity(),
		District:      req.GetDistrict(),
		AddressLine:   req.GetAddressLine(),
		PostalCode:    req.GetPostalCode(),
	})
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &orderv1.CreateOrderResponse{Order: orderToProto(order)}, nil
}

// GetOrder 查询当前用户的单笔订单 RPC。
func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetOrderId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 必须为正数")).Err()
	}

	order, err := h.orderUC.GetOrderByID(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &orderv1.GetOrderResponse{Order: orderToProto(order)}, nil
}

// ListOrders 分页查询当前用户订单列表 RPC。
func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}

	items, total, err := h.orderUC.ListOrdersByUserID(ctx, req.GetUserId(), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}

	pbItems := make([]*orderv1.Order, 0, len(items))
	for _, item := range items {
		pbItems = append(pbItems, orderToProto(item))
	}
	return &orderv1.ListOrdersResponse{
		Items: pbItems,
		Total: total,
	}, nil
}

// CancelOrder 取消当前用户订单 RPC。
func (h *OrderHandler) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "user_id 必须为正数")).Err()
	}
	if req.GetOrderId() <= 0 {
		return nil, pkgerrors.ToGRPCStatus(pkgerrors.New(pkgerrors.CodeInvalidArg, "order_id 必须为正数")).Err()
	}

	order, err := h.orderUC.CancelOrder(ctx, req.GetUserId(), req.GetOrderId())
	if err != nil {
		return nil, pkgerrors.ToGRPCStatus(err).Err()
	}
	return &orderv1.CancelOrderResponse{Order: orderToProto(order)}, nil
}

// orderToProto 将领域订单聚合转换为 proto 响应对象。
func orderToProto(order *domain.Order) *orderv1.Order {
	if order == nil {
		return nil
	}
	items := make([]*orderv1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &orderv1.OrderItem{
			Id:            item.ID,
			OrderId:       item.OrderID,
			ProductId:     item.ProductID,
			SkuId:         item.SKUID,
			SkuCode:       item.SKUCode,
			SkuTitle:      item.SKUTitle,
			Name:          item.Name,
			ImageUrl:      item.ImageURL,
			OptionSummary: item.OptionSummary,
			Price:         item.Price,
			Quantity:      item.Quantity,
			Amount:        item.Amount,
		})
	}
	return &orderv1.Order{
		Id:            order.ID,
		OrderNo:       order.OrderNo,
		UserId:        order.UserID,
		RequestId:     order.RequestID,
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
		CreatedAt:     order.CreatedAt.Unix(),
		UpdatedAt:     order.UpdatedAt.Unix(),
		Items:         items,
	}
}
