package integration

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yym108/gobao-pkg/idempotency"
)

// IdempotencyStore 复用 gobao-pkg 的 Redis 幂等守卫，实现订单应用层的幂等存储抽象。
type IdempotencyStore struct {
	guard *idempotency.Guard // 底层 Redis SETNX 守卫
}

// NewIdempotencyStore 创建订单幂等存储适配器。
func NewIdempotencyStore(rdb *redis.Client, prefix string) *IdempotencyStore {
	return &IdempotencyStore{guard: idempotency.New(rdb, prefix)}
}

// Acquire 尝试占用幂等键。
func (s *IdempotencyStore) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.guard.Acquire(ctx, key, ttl)
}
