package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-order/internal/adapter/integration"
)

// TestIdempotencyStore_Acquire 验证订单适配层会复用 Redis SETNX 幂等守卫。
func TestIdempotencyStore_Acquire(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := integration.NewIdempotencyStore(rdb, "order:req:")
	first, err := store.Acquire(context.Background(), "1001:req-001", 10*time.Minute)
	require.NoError(t, err)
	assert.True(t, first)

	second, err := store.Acquire(context.Background(), "1001:req-001", 10*time.Minute)
	require.NoError(t, err)
	assert.False(t, second)
}
