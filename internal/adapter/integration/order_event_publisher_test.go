package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-order/internal/adapter/integration"
	"github.com/yym108/gobao-order/internal/domain"
	"github.com/yym108/gobao-pkg/mq"
)

type orderEventPayload struct {
	ID          int64  `json:"id"`
	OrderNo     string `json:"order_no"`
	UserID      int64  `json:"user_id"`
	TotalAmount int64  `json:"total_amount"`
	Status      string `json:"status"`
}

// TestOrderEventPublisher_PublishCreatedAndCancelled 验证订单事件发布器会按约定主题投递创建与取消事件。
func TestOrderEventPublisher_PublishCreatedAndCancelled(t *testing.T) {
	url := runEmbeddedNATS(t)
	bus, err := mq.New(mq.Config{
		URL:      url,
		Stream:   "ORDER_TEST",
		Subjects: []string{"order.created", "order.cancelled"},
	})
	require.NoError(t, err)
	t.Cleanup(bus.Close)

	publisher := integration.NewOrderEventPublisher(bus, "order.created", "order.cancelled")

	createdCh := make(chan orderEventPayload, 1)
	cancelledCh := make(chan orderEventPayload, 1)

	err = bus.Subscribe(context.Background(), "order-created-test", "order.created", func(_ context.Context, payload []byte) error {
		var got orderEventPayload
		if err := json.Unmarshal(payload, &got); err != nil {
			return err
		}
		createdCh <- got
		return nil
	})
	require.NoError(t, err)

	err = bus.Subscribe(context.Background(), "order-cancelled-test", "order.cancelled", func(_ context.Context, payload []byte) error {
		var got orderEventPayload
		if err := json.Unmarshal(payload, &got); err != nil {
			return err
		}
		cancelledCh <- got
		return nil
	})
	require.NoError(t, err)

	order := &domain.Order{
		ID:            101,
		OrderNo:       "ORD-101",
		UserID:        1001,
		RequestID:     "req-101",
		Status:        domain.OrderStatusCreated,
		TotalAmount:   999900,
		TotalQuantity: 1,
		CreatedAt:     time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC),
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
				Quantity:      1,
				Amount:        999900,
			},
		},
	}

	require.NoError(t, publisher.PublishOrderCreated(context.Background(), order))
	select {
	case payload := <-createdCh:
		assert.Equal(t, int64(101), payload.ID)
		assert.Equal(t, "ORD-101", payload.OrderNo)
		assert.Equal(t, int64(1001), payload.UserID)
		assert.Equal(t, int64(999900), payload.TotalAmount)
		assert.Equal(t, "CREATED", payload.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting created event")
	}

	order.Status = domain.OrderStatusCancelled
	require.NoError(t, publisher.PublishOrderCancelled(context.Background(), order))
	select {
	case payload := <-cancelledCh:
		assert.Equal(t, int64(101), payload.ID)
		assert.Equal(t, "ORD-101", payload.OrderNo)
		assert.Equal(t, "CANCELLED", payload.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting cancelled event")
	}
}

// runEmbeddedNATS 在进程内启动一个带 JetStream 的 NATS server。
func runEmbeddedNATS(t *testing.T) string {
	t.Helper()
	opts := &server.Options{
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(2*time.Second))
	t.Cleanup(s.Shutdown)
	return s.ClientURL()
}
