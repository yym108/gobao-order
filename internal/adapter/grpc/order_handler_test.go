package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/yym108/gobao-order/internal/application"
	"github.com/yym108/gobao-order/internal/domain"
	pkgerrors "github.com/yym108/gobao-pkg/errors"
	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
)

// mockOrderUseCase 用 function field 模拟订单应用层，便于验证 gRPC 层映射逻辑。
type mockOrderUseCase struct {
	createOrderFn      func(ctx context.Context, cmd application.CreateOrderCommand) (*domain.Order, error)
	getOrderByIDFn     func(ctx context.Context, userID, orderID int64) (*domain.Order, error)
	listOrdersByUserFn func(ctx context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error)
	cancelOrderFn      func(ctx context.Context, userID, orderID int64) (*domain.Order, error)
}

// CreateOrder 执行测试桩定义的下单行为。
func (m *mockOrderUseCase) CreateOrder(ctx context.Context, cmd application.CreateOrderCommand) (*domain.Order, error) {
	return m.createOrderFn(ctx, cmd)
}

// GetOrderByID 执行测试桩定义的单笔查单行为。
func (m *mockOrderUseCase) GetOrderByID(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	return m.getOrderByIDFn(ctx, userID, orderID)
}

// ListOrdersByUserID 执行测试桩定义的分页列单行为。
func (m *mockOrderUseCase) ListOrdersByUserID(ctx context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error) {
	return m.listOrdersByUserFn(ctx, userID, page, pageSize)
}

// CancelOrder 执行测试桩定义的取消订单行为。
func (m *mockOrderUseCase) CancelOrder(ctx context.Context, userID, orderID int64) (*domain.Order, error) {
	return m.cancelOrderFn(ctx, userID, orderID)
}

// setupOrderBufconn 创建仅承载订单服务的 bufconn 测试环境。
func setupOrderBufconn(t *testing.T, orderUC *mockOrderUseCase) orderv1.OrderServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(srv, NewOrderHandler(orderUC))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return orderv1.NewOrderServiceClient(conn)
}

// TestCreateOrder_Success 验证创建订单 RPC 能正确透传请求并映射响应。
func TestCreateOrder_Success(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, cmd application.CreateOrderCommand) (*domain.Order, error) {
			assert.Equal(t, int64(1001), cmd.UserID)
			assert.Equal(t, "req-001", cmd.RequestID)
			assert.Equal(t, int64(1001002), cmd.SKUID)
			assert.Equal(t, int32(2), cmd.Quantity)
			assert.Equal(t, "张三", cmd.ReceiverName)
			return &domain.Order{
				ID:            101,
				OrderNo:       "ORD-20260518123000-1001",
				UserID:        cmd.UserID,
				RequestID:     cmd.RequestID,
				Status:        domain.OrderStatusCreated,
				TotalAmount:   1999800,
				TotalQuantity: 2,
				ReceiverName:  cmd.ReceiverName,
				ReceiverPhone: cmd.ReceiverPhone,
				Province:      cmd.Province,
				City:          cmd.City,
				District:      cmd.District,
				AddressLine:   cmd.AddressLine,
				PostalCode:    cmd.PostalCode,
				CreatedAt:     time.Date(2026, 5, 18, 12, 30, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2026, 5, 18, 12, 30, 0, 0, time.UTC),
				Items: []domain.OrderItem{
					{
						ID:            1,
						OrderID:       101,
						ProductID:     1001,
						SKUID:         1001002,
						SKUCode:       "MBA-M4-16G-512G",
						SKUTitle:      "MacBook Air 13 英寸 M4 / 16GB / 512GB",
						Name:          "MacBook Air",
						ImageURL:      "https://example.com/macbook-air.png",
						OptionSummary: "M4 / 16GB / 512GB",
						Price:         999900,
						Quantity:      2,
						Amount:        1999800,
					},
				},
			}, nil
		},
	})

	resp, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId:        1001,
		RequestId:     "req-001",
		SkuId:         1001002,
		Quantity:      2,
		ReceiverName:  "张三",
		ReceiverPhone: "13800138000",
		Province:      "上海市",
		City:          "上海市",
		District:      "浦东新区",
		AddressLine:   "世纪大道 100 号 18 层",
		PostalCode:    "200120",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(101), resp.GetOrder().GetId())
	assert.Equal(t, "ORD-20260518123000-1001", resp.GetOrder().GetOrderNo())
	assert.Len(t, resp.GetOrder().GetItems(), 1)
	assert.Equal(t, int64(1001002), resp.GetOrder().GetItems()[0].GetSkuId())
	assert.Equal(t, "MBA-M4-16G-512G", resp.GetOrder().GetItems()[0].GetSkuCode())
}

// TestCreateOrder_InvalidArgument 验证无效入参会在 gRPC 层被直接拦截。
func TestCreateOrder_InvalidArgument(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, cmd application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("should not call use case on invalid request")
			return nil, nil
		},
	})

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId:    1001,
		RequestId: "",
		SkuId:     1001002,
		Quantity:  1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateOrder_Conflict 验证应用层冲突错误能映射为 AlreadyExists。
func TestCreateOrder_Conflict(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, cmd application.CreateOrderCommand) (*domain.Order, error) {
			return nil, pkgerrors.New(pkgerrors.CodeConflict, "重复下单请求")
		},
	})

	_, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId:    1001,
		RequestId: "req-dup",
		SkuId:     1001002,
		Quantity:  1,
	})
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

// TestGetOrder_Success 验证按订单 ID 查单时会正确透传用户身份并映射响应。
func TestGetOrder_Success(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, _ application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderByIDFn: func(_ context.Context, userID, orderID int64) (*domain.Order, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(101), orderID)
			return &domain.Order{
				ID:            101,
				OrderNo:       "ORD-20260518143000-1001",
				UserID:        1001,
				RequestID:     "req-101",
				Status:        domain.OrderStatusCreated,
				TotalAmount:   999900,
				TotalQuantity: 1,
				ReceiverName:  "张三",
				CreatedAt:     time.Date(2026, 5, 18, 14, 30, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2026, 5, 18, 14, 30, 0, 0, time.UTC),
				Items: []domain.OrderItem{
					{ID: 1, OrderID: 101, SKUID: 1001002, SKUCode: "MBA-M4-16G-512G", Name: "MacBook Air", Price: 999900, Quantity: 1, Amount: 999900},
				},
			}, nil
		},
		listOrdersByUserFn: func(_ context.Context, _ int64, _ int, _ int) ([]*domain.Order, int64, error) {
			t.Fatal("unexpected ListOrdersByUserID call")
			return nil, 0, nil
		},
	})

	resp, err := client.GetOrder(context.Background(), &orderv1.GetOrderRequest{
		UserId:  1001,
		OrderId: 101,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(101), resp.GetOrder().GetId())
	assert.Equal(t, "ORD-20260518143000-1001", resp.GetOrder().GetOrderNo())
	assert.Len(t, resp.GetOrder().GetItems(), 1)
}

// TestGetOrder_Forbidden 验证应用层无权访问错误会映射为 PermissionDenied。
func TestGetOrder_Forbidden(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, _ application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderByIDFn: func(_ context.Context, _, _ int64) (*domain.Order, error) {
			return nil, pkgerrors.New(pkgerrors.CodeForbidden, "无权访问该订单")
		},
		listOrdersByUserFn: func(_ context.Context, _ int64, _ int, _ int) ([]*domain.Order, int64, error) {
			t.Fatal("unexpected ListOrdersByUserID call")
			return nil, 0, nil
		},
	})

	_, err := client.GetOrder(context.Background(), &orderv1.GetOrderRequest{
		UserId:  1001,
		OrderId: 102,
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestListOrders_Success 验证分页列单 RPC 能正确映射列表与总数。
func TestListOrders_Success(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, _ application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderByIDFn: func(_ context.Context, _, _ int64) (*domain.Order, error) {
			t.Fatal("unexpected GetOrderByID call")
			return nil, nil
		},
		listOrdersByUserFn: func(_ context.Context, userID int64, page, pageSize int) ([]*domain.Order, int64, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, 1, page)
			assert.Equal(t, 2, pageSize)
			return []*domain.Order{
				{ID: 103, OrderNo: "ORD-003", UserID: 1001, Status: domain.OrderStatusCreated, TotalAmount: 1999800, TotalQuantity: 2},
				{ID: 102, OrderNo: "ORD-002", UserID: 1001, Status: domain.OrderStatusCreated, TotalAmount: 999900, TotalQuantity: 1},
			}, 3, nil
		},
	})

	resp, err := client.ListOrders(context.Background(), &orderv1.ListOrdersRequest{
		UserId:   1001,
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.GetTotal())
	assert.Len(t, resp.GetItems(), 2)
	assert.Equal(t, "ORD-003", resp.GetItems()[0].GetOrderNo())
	assert.Equal(t, "ORD-002", resp.GetItems()[1].GetOrderNo())
}

// TestCancelOrder_Success 验证取消订单 RPC 会正确透传用户身份并返回取消后的订单。
func TestCancelOrder_Success(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, _ application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderByIDFn: func(_ context.Context, _, _ int64) (*domain.Order, error) {
			t.Fatal("unexpected GetOrderByID call")
			return nil, nil
		},
		listOrdersByUserFn: func(_ context.Context, _ int64, _ int, _ int) ([]*domain.Order, int64, error) {
			t.Fatal("unexpected ListOrdersByUserID call")
			return nil, 0, nil
		},
		cancelOrderFn: func(_ context.Context, userID, orderID int64) (*domain.Order, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(201), orderID)
			return &domain.Order{
				ID:      201,
				OrderNo: "ORD-CANCEL-201",
				UserID:  1001,
				Status:  domain.OrderStatusCancelled,
			}, nil
		},
	})

	resp, err := client.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{
		UserId:  1001,
		OrderId: 201,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(201), resp.GetOrder().GetId())
	assert.Equal(t, domain.OrderStatusCancelled, resp.GetOrder().GetStatus())
}

// TestCancelOrder_FailedPrecondition 验证不可取消状态会映射为 FailedPrecondition。
func TestCancelOrder_FailedPrecondition(t *testing.T) {
	client := setupOrderBufconn(t, &mockOrderUseCase{
		createOrderFn: func(_ context.Context, _ application.CreateOrderCommand) (*domain.Order, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderByIDFn: func(_ context.Context, _, _ int64) (*domain.Order, error) {
			t.Fatal("unexpected GetOrderByID call")
			return nil, nil
		},
		listOrdersByUserFn: func(_ context.Context, _ int64, _ int, _ int) ([]*domain.Order, int64, error) {
			t.Fatal("unexpected ListOrdersByUserID call")
			return nil, 0, nil
		},
		cancelOrderFn: func(_ context.Context, _, _ int64) (*domain.Order, error) {
			return nil, pkgerrors.New(pkgerrors.CodeFailedPrecondition, "当前订单状态不可取消")
		},
	})

	_, err := client.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{
		UserId:  1001,
		OrderId: 202,
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}
