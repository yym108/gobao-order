# gobao-order

GoBao 的订单服务仓库，当前已经承担订单创建、查询、取消与事件发布链路。

## 作用

- 订单创建、订单列表、订单详情、取消订单
- 与商品服务对接 SKU / 库存校验
- 订单事件发布与支付结果消费
- 秒杀下单事件消费

## 关系

- 依赖 `gobao-proto`、`gobao-pkg`
- 调用 `gobao-product` 做 SKU / 库存联动
- 被 `gobao-gateway` 调用

## 独立使用前准备

单独 clone 本仓后，先执行：

```bash
bash scripts/bootstrap-deps.sh
ln -sfn workspace/gobao-pkg ../gobao-pkg
ln -sfn workspace/gobao-proto ../gobao-proto
```

## 环境变量

可参考仓库根目录 `.env.example`：

- `ORDER_MYSQL_DSN`
- `ORDER_PRODUCT_GRPC_ADDR`
- `ORDER_REDIS_ADDR`
- `ORDER_NATS_URL`
- `ORDER_ORDER_CREATED_SUBJECT`
- `ORDER_PAYMENT_FAILED_SUBJECT`

## 启动

```bash
go test ./...
go run ./cmd/server
```

如需容器化启动，可直接使用仓库内 `Dockerfile`，或由 `gobao-deploy` / `GoBao` 主仓统一编排。
