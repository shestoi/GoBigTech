package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/shestoi/GoBigTech/services/order/internal/repository/mocks"
)

func TestOrderService_HandleOrderAssemblyCompleted(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	event := OrderAssemblyCompletedEvent{
		EventID:      "evt-1",
		EventType:    "order.assembly.completed",
		EventVersion: 1,
		OccurredAt:   time.Now(),
		OrderID:      "order-123",
		UserID:       "user-456",
	}

	t.Run("inserted=true, rowsAffected=1 -> ok", func(t *testing.T) {
		mockRepo := mocks.NewOrderRepository(t)
		svc := NewOrderService(logger, nil, nil, mockRepo, "order.payment.completed", nil)

		mockRepo.On("HandleAssemblyCompletedTx", ctx, "evt-1", "order.assembly.completed", event.OccurredAt, "order-123").
			Return(true, int64(1), nil).Once()

		err := svc.HandleOrderAssemblyCompleted(ctx, event)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("inserted=false (duplicate) -> ok, update not required", func(t *testing.T) {
		mockRepo := mocks.NewOrderRepository(t)
		svc := NewOrderService(logger, nil, nil, mockRepo, "order.payment.completed", nil)

		mockRepo.On("HandleAssemblyCompletedTx", ctx, "evt-1", "order.assembly.completed", event.OccurredAt, "order-123").
			Return(false, int64(0), nil).Once()

		err := svc.HandleOrderAssemblyCompleted(ctx, event)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("inserted=true, rowsAffected=0 -> ok + warn", func(t *testing.T) {
		mockRepo := mocks.NewOrderRepository(t)
		svc := NewOrderService(logger, nil, nil, mockRepo, "order.payment.completed", nil)

		mockRepo.On("HandleAssemblyCompletedTx", ctx, "evt-1", "order.assembly.completed", event.OccurredAt, "order-123").
			Return(true, int64(0), nil).Once()

		err := svc.HandleOrderAssemblyCompleted(ctx, event)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("repo error -> error", func(t *testing.T) {
		mockRepo := mocks.NewOrderRepository(t)
		svc := NewOrderService(logger, nil, nil, mockRepo, "order.payment.completed", nil)

		repoErr := errors.New("repository error")
		mockRepo.On("HandleAssemblyCompletedTx", ctx, "evt-1", "order.assembly.completed", event.OccurredAt, "order-123").
			Return(false, int64(0), repoErr).Once()

		err := svc.HandleOrderAssemblyCompleted(ctx, event)
		assert.Error(t, err)
		assert.Equal(t, repoErr, err)

		mockRepo.AssertExpectations(t)
	})
}
