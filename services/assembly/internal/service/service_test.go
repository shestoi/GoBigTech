package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockSleeper реализует Sleeper для тестов (не ждёт реального времени)
type MockSleeper struct{}

func (m *MockSleeper) Sleep(ctx context.Context, d time.Duration) error {
	return nil // сразу возвращаемся, не ждём
}

// MockAssemblyEventPublisher реализует AssemblyEventPublisher для тестов (избегаем цикла импортов)
type MockAssemblyEventPublisher struct {
	mock.Mock
}

func (m *MockAssemblyEventPublisher) PublishOrderAssemblyCompleted(ctx context.Context, event OrderAssemblyCompletedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestService_HandleOrderPaid_Idempotency(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Создаём моки
	mockPublisher := new(MockAssemblyEventPublisher)
	mockStore := new(MockProcessedEventsStore)
	mockSleeper := &MockSleeper{}

	// Создаём сервис с mock sleeper (чтобы не ждать 10 секунд)
	svc := NewServiceWithSleeper(logger, mockPublisher, mockStore, mockSleeper, 24*time.Hour)

	event := OrderPaidEvent{
		EventID:       "evt-1",
		EventType:     "order.payment.completed",
		EventVersion:  1,
		OccurredAt:    time.Now(),
		OrderID:       "order-123",
		UserID:        "user-456",
		Amount:        10000,
		PaymentMethod: "card",
	}

	t.Run("first call should process event", func(t *testing.T) {
		// Первый вызов: событие не обработано
		mockStore.On("IsProcessed", ctx, "evt-1").Return(false, nil).Once()
		// Используем mock.MatchedBy для проверки типа события
		mockPublisher.On("PublishOrderAssemblyCompleted", ctx, mock.MatchedBy(func(e OrderAssemblyCompletedEvent) bool {
			return e.OrderID == "order-123" && e.UserID == "user-456"
		})).Return(nil).Once()
		mockStore.On("MarkProcessed", ctx, "evt-1", 24*time.Hour).Return(nil).Once()

		err := svc.HandleOrderPaid(ctx, event)
		assert.NoError(t, err)

		mockPublisher.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("second call with same event_id should skip processing", func(t *testing.T) {
		// Второй вызов: событие уже обработано
		mockStore.On("IsProcessed", ctx, "evt-1").Return(true, nil).Once()
		// PublishOrderAssemblyCompleted НЕ должен вызываться
		// MarkProcessed НЕ должен вызываться

		err := svc.HandleOrderPaid(ctx, event)
		assert.NoError(t, err)

		mockPublisher.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})
}

// MockProcessedEventsStore реализует ProcessedEventsStore для тестов
type MockProcessedEventsStore struct {
	mock.Mock
}

func (m *MockProcessedEventsStore) MarkProcessed(ctx context.Context, eventID string, ttl time.Duration) error {
	args := m.Called(ctx, eventID, ttl)
	return args.Error(0)
}

func (m *MockProcessedEventsStore) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}

func TestService_HandleOrderPaid_EventIDRequired(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	mockPublisher := new(MockAssemblyEventPublisher)
	mockStore := new(MockProcessedEventsStore)
	mockSleeper := &MockSleeper{}

	svc := NewServiceWithSleeper(logger, mockPublisher, mockStore, mockSleeper, 24*time.Hour)

	event := OrderPaidEvent{
		EventID:       "", // отсутствует event_id
		EventType:     "order.payment.completed",
		OrderID:       "order-123",
		UserID:        "user-456",
		Amount:        10000,
		PaymentMethod: "card",
	}

	err := svc.HandleOrderPaid(ctx, event)
	assert.Error(t, err)
	assert.Equal(t, ErrEventIDRequired, err)

	// Никакие методы не должны вызываться
	mockPublisher.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestService_HandleOrderPaid_StoreError(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	mockPublisher := new(MockAssemblyEventPublisher)
	mockStore := new(MockProcessedEventsStore)
	mockSleeper := &MockSleeper{}

	svc := NewServiceWithSleeper(logger, mockPublisher, mockStore, mockSleeper, 24*time.Hour)

	event := OrderPaidEvent{
		EventID:       "evt-1",
		EventType:     "order.payment.completed",
		OrderID:       "order-123",
		UserID:        "user-456",
		Amount:        10000,
		PaymentMethod: "card",
	}

	// Ошибка при проверке IsProcessed
	storeErr := errors.New("store error")
	mockStore.On("IsProcessed", ctx, "evt-1").Return(false, storeErr).Once()

	err := svc.HandleOrderPaid(ctx, event)
	assert.Error(t, err)
	assert.Equal(t, storeErr, err)

	mockPublisher.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestService_HandleOrderPaid_PublisherError(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	mockPublisher := new(MockAssemblyEventPublisher)
	mockStore := new(MockProcessedEventsStore)
	mockSleeper := &MockSleeper{}

	svc := NewServiceWithSleeper(logger, mockPublisher, mockStore, mockSleeper, 24*time.Hour)

	event := OrderPaidEvent{
		EventID:       "evt-1",
		EventType:     "order.payment.completed",
		OrderID:       "order-123",
		UserID:        "user-456",
		Amount:        10000,
		PaymentMethod: "card",
	}

	publisherErr := errors.New("publisher error")
	mockStore.On("IsProcessed", ctx, "evt-1").Return(false, nil).Once()
	mockPublisher.On("PublishOrderAssemblyCompleted", ctx, mock.MatchedBy(func(e OrderAssemblyCompletedEvent) bool {
		return e.OrderID == "order-123" && e.UserID == "user-456"
	})).Return(publisherErr).Once()

	err := svc.HandleOrderPaid(ctx, event)
	assert.Error(t, err)
	assert.Equal(t, publisherErr, err)

	// MarkProcessed не должен вызываться при ошибке publisher
	mockPublisher.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
