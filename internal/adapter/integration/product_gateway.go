// Package integration 提供 Order 服务对外部依赖的接线适配实现。
// 当前主要承接 Product gRPC 和 Redis 幂等守卫，供 application 层通过抽象接口调用。
package integration

import (
	"context"
	"fmt"

	"github.com/yym108/gobao-order/internal/application"
	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProductGateway 基于 Product gRPC 实现订单应用层所需的商品协作接口。
type ProductGateway struct {
	conn   *grpc.ClientConn               // Product gRPC 连接
	client productv1.ProductServiceClient // proto 生成的 Product client
}

// NewProductGateway 创建 Product gRPC 适配器。
func NewProductGateway(addr string) (*ProductGateway, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial product: %w", err)
	}
	return &ProductGateway{
		conn:   conn,
		client: productv1.NewProductServiceClient(conn),
	}, nil
}

// Close 关闭底层 gRPC 连接。
func (g *ProductGateway) Close() error {
	return g.conn.Close()
}

// GetSKUByID 按 sku_id 查询后端权威 SKU 快照。
// 当前 Product 侧只有商品详情 RPC，因此这里会先按商品 ID 查询详情，再在返回的 SKU 列表中定位目标项。
func (g *ProductGateway) GetSKUByID(ctx context.Context, skuID int64) (*application.SKUView, error) {
	productID := skuID / 1000
	if productID <= 0 {
		return nil, nil
	}
	resp, err := g.client.GetProduct(ctx, &productv1.GetProductRequest{Id: productID})
	if err != nil {
		return nil, err
	}
	product := resp.GetProduct()
	if product == nil {
		return nil, nil
	}
	for _, sku := range resp.GetSkus() {
		if sku.GetSkuId() != skuID {
			continue
		}
		return &application.SKUView{
			ProductID:      product.GetId(),
			SKUID:          sku.GetSkuId(),
			SKUCode:        sku.GetSkuCode(),
			Title:          sku.GetTitle(),
			ProductName:    product.GetName(),
			ImageURL:       product.GetImageUrl(),
			OptionSummary:  sku.GetOptionSummary(),
			OptionValueIDs: append([]int64(nil), sku.GetOptionValueIds()...),
			Price:          sku.GetPrice(),
			StockQuantity:  sku.GetStockQuantity(),
			Status:         sku.GetStatus(),
		}, nil
	}
	return nil, nil
}

// DeductStock 调用 Product 内部库存扣减 RPC。
func (g *ProductGateway) DeductStock(ctx context.Context, productID int64, quantity int32) error {
	_, err := g.client.DeductStock(ctx, &productv1.DeductStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})
	return err
}

// RestoreStock 调用 Product 内部库存回补 RPC。
func (g *ProductGateway) RestoreStock(ctx context.Context, productID int64, quantity int32) error {
	_, err := g.client.RestoreStock(ctx, &productv1.RestoreStockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})
	return err
}
