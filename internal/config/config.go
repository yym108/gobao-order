// Package config 定义 Order 服务的配置结构。
// 通过 mapstructure tag 支持从环境变量加载（前缀 ORDER_）。
package config

// Config 是 Order 服务当前阶段的完整配置。
// I3 起在保留消息链路的同时补齐订单持久化接线，方便逐步从 mock 过渡到真实订单服务。
type Config struct {
	HTTPAddr              string `mapstructure:"http_addr"`               // HTTP 监听地址，如 ":8080"
	GRPCAddr              string `mapstructure:"grpc_addr"`               // gRPC 监听地址，如 ":9090"
	LogLevel              string `mapstructure:"log_level"`               // 日志级别：debug/info/warn/error
	MySQLDSN              string `mapstructure:"mysql_dsn"`               // MySQL 连接串，用于订单聚合持久化
	ProductGRPCAddr       string `mapstructure:"product_grpc_addr"`       // Product 服务 gRPC 地址，用于读取 SKU 与扣减库存
	RedisAddr             string `mapstructure:"redis_addr"`              // Redis 地址，用于订单创建幂等控制
	RedisDB               int    `mapstructure:"redis_db"`                // Redis 数据库编号，默认使用 0
	NATSURL               string `mapstructure:"nats_url"`                // NATS 连接地址，用于订阅秒杀下单事件
	NATSStream            string `mapstructure:"nats_stream"`             // JetStream 流名称，如 "SECKILL"
	SeckillOrderSubject   string `mapstructure:"seckill_order_subject"`   // 秒杀下单主题，如 "seckill.order"
	SeckillOrderConsumer  string `mapstructure:"seckill_order_consumer"`  // 秒杀下单消费者名称，保证重启后可延续消费位点
	OrderCreatedSubject   string `mapstructure:"order_created_subject"`   // 订单创建完成后的事件主题，供 Payment mock 后续订阅
	OrderCancelledSubject string `mapstructure:"order_cancelled_subject"` // 订单取消事件主题，为后续超时关单与库存回补预留
	PaymentPaidSubject    string `mapstructure:"payment_paid_subject"`    // 支付成功事件主题，供订单服务推进已支付状态
	PaymentFailedSubject  string `mapstructure:"payment_failed_subject"`  // 支付失败事件主题，供订单服务推进支付失败状态
}
