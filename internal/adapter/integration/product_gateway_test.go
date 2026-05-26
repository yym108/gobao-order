package integration_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-order/internal/adapter/integration"
	"github.com/yym108/gobao-order/internal/application"
	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
	"google.golang.org/grpc"
)

// fakeProductService 使用函数桩模拟 Product gRPC 服务，验证 Order 侧适配逻辑。
type fakeProductService struct {
	productv1.UnimplementedProductServiceServer
	getProductFn   func(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error)
	deductStockFn  func(ctx context.Context, req *productv1.DeductStockRequest) (*productv1.DeductStockResponse, error)
	restoreStockFn func(ctx context.Context, req *productv1.RestoreStockRequest) (*productv1.RestoreStockResponse, error)
}

// GetProduct 返回测试桩定义的商品详情。
func (s *fakeProductService) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	return s.getProductFn(ctx, req)
}

// DeductStock 返回测试桩定义的库存扣减结果。
func (s *fakeProductService) DeductStock(ctx context.Context, req *productv1.DeductStockRequest) (*productv1.DeductStockResponse, error) {
	return s.deductStockFn(ctx, req)
}

// RestoreStock 返回测试桩定义的库存回补结果。
func (s *fakeProductService) RestoreStock(ctx context.Context, req *productv1.RestoreStockRequest) (*productv1.RestoreStockResponse, error) {
	return s.restoreStockFn(ctx, req)
}

// TestProductGateway_GetSKUByID 验证 Order 会从商品详情中按 sku_id 解析出后端权威 SKU 快照。
func TestProductGateway_GetSKUByID(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcServer, &fakeProductService{
		getProductFn: func(_ context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
			assert.Equal(t, int64(1001), req.GetId())
			return &productv1.GetProductResponse{
				Product: &productv1.Product{
					Id:       1001,
					Name:     "MacBook Air",
					ImageUrl: "https://example.com/macbook-air.png",
				},
				Skus: []*productv1.ProductSku{
					{
						SkuId:          1001001,
						SkuCode:        "MBA-M4-16G-256G",
						Title:          "MacBook Air 13 英寸 M4 / 16GB / 256GB",
						OptionSummary:  "M4 / 16GB / 256GB",
						OptionValueIds: []int64{101, 201, 301},
						Price:          849900,
						StockQuantity:  10,
						Status:         1,
					},
					{
						SkuId:          1001002,
						SkuCode:        "MBA-M4-16G-512G",
						Title:          "MacBook Air 13 英寸 M4 / 16GB / 512GB",
						OptionSummary:  "M4 / 16GB / 512GB",
						OptionValueIds: []int64{101, 201, 302},
						Price:          999900,
						StockQuantity:  8,
						Status:         1,
					},
				},
			}, nil
		},
		deductStockFn: func(_ context.Context, req *productv1.DeductStockRequest) (*productv1.DeductStockResponse, error) {
			return &productv1.DeductStockResponse{Remaining: 7}, nil
		},
		restoreStockFn: func(_ context.Context, req *productv1.RestoreStockRequest) (*productv1.RestoreStockResponse, error) {
			return &productv1.RestoreStockResponse{Remaining: 9}, nil
		},
	})
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	gateway, err := integration.NewProductGateway(lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = gateway.Close() })

	got, err := gateway.GetSKUByID(context.Background(), 1001002)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, &application.SKUView{
		ProductID:      1001,
		SKUID:          1001002,
		SKUCode:        "MBA-M4-16G-512G",
		Title:          "MacBook Air 13 英寸 M4 / 16GB / 512GB",
		ProductName:    "MacBook Air",
		ImageURL:       "https://example.com/macbook-air.png",
		OptionSummary:  "M4 / 16GB / 512GB",
		OptionValueIDs: []int64{101, 201, 302},
		Price:          999900,
		StockQuantity:  8,
		Status:         1,
	}, got)
}

// TestProductGateway_DeductAndRestoreStock 验证库存协作直接透传到 Product 内部库存 RPC。
func TestProductGateway_DeductAndRestoreStock(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcServer, &fakeProductService{
		getProductFn: func(_ context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
			return &productv1.GetProductResponse{}, nil
		},
		deductStockFn: func(_ context.Context, req *productv1.DeductStockRequest) (*productv1.DeductStockResponse, error) {
			assert.Equal(t, int64(1001), req.GetProductId())
			assert.Equal(t, int32(2), req.GetQuantity())
			return &productv1.DeductStockResponse{Remaining: 6}, nil
		},
		restoreStockFn: func(_ context.Context, req *productv1.RestoreStockRequest) (*productv1.RestoreStockResponse, error) {
			assert.Equal(t, int64(1001), req.GetProductId())
			assert.Equal(t, int32(2), req.GetQuantity())
			return &productv1.RestoreStockResponse{Remaining: 8}, nil
		},
	})
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	gateway, err := integration.NewProductGateway(lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = gateway.Close() })

	require.NoError(t, gateway.DeductStock(context.Background(), 1001, 2))
	require.NoError(t, gateway.RestoreStock(context.Background(), 1001, 2))
}
