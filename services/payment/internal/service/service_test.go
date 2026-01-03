package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shestoi/GoBigTech/services/payment/internal/repository"
	"github.com/shestoi/GoBigTech/services/payment/internal/repository/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPaymentService_ProcessPayment(t *testing.T) {
	ctx := context.Background()

	t.Run("amount <= 0 returns error, repo not called", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-1", "user-1", 0, "card")

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid amount")
		require.False(t, success)
		require.Empty(t, transactionID)
		mockRepo.AssertNotCalled(t, "GetByOrderID")
		mockRepo.AssertNotCalled(t, "Save")
	})

	t.Run("negative amount returns error, repo not called", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-1", "user-1", -10.0, "card")

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid amount")
		require.False(t, success)
		require.Empty(t, transactionID)
		mockRepo.AssertNotCalled(t, "GetByOrderID")
		mockRepo.AssertNotCalled(t, "Save")
	})

	t.Run("existing transaction returns same transactionID, Save not called", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		existingTx := repository.Transaction{
			OrderID:       "order-1",
			UserID:        "user-1",
			Amount:        100.0,
			Method:        "card",
			TransactionID: "tx_order-1_1234567890",
			Status:        "success",
			CreatedAt:     time.Now().Unix(),
		}

		mockRepo.On("GetByOrderID", ctx, "order-1").Return(existingTx, nil).Once()

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-1", "user-1", 100.0, "card")

		// Assert
		require.NoError(t, err)
		require.True(t, success)
		require.Equal(t, "tx_order-1_1234567890", transactionID)
		mockRepo.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "Save")
	})

	t.Run("ErrNotFound creates new transaction and saves it", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		mockRepo.On("GetByOrderID", ctx, "order-2").Return(repository.Transaction{}, repository.ErrNotFound).Once()
		mockRepo.On("Save", ctx, mock.MatchedBy(func(tx repository.Transaction) bool {
			return tx.OrderID == "order-2" &&
				tx.UserID == "user-2" &&
				tx.Amount == 200.0 &&
				tx.Method == "card" &&
				tx.Status == "success" &&
				tx.TransactionID != "" &&
				tx.CreatedAt > 0
		})).Return(nil).Once()

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-2", "user-2", 200.0, "card")

		// Assert
		require.NoError(t, err)
		require.True(t, success)
		require.NotEmpty(t, transactionID)
		require.Contains(t, transactionID, "tx_order-2_")
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetByOrderID returns arbitrary error", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		arbitraryErr := errors.New("database connection failed")
		mockRepo.On("GetByOrderID", ctx, "order-3").Return(repository.Transaction{}, arbitraryErr).Once()

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-3", "user-3", 300.0, "card")

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to check existing transaction")
		require.False(t, success)
		require.Empty(t, transactionID)
		mockRepo.AssertExpectations(t)
		mockRepo.AssertNotCalled(t, "Save")
	})

	t.Run("Save returns error", func(t *testing.T) {
		// Arrange
		mockRepo := mocks.NewPaymentRepository(t)
		service := NewPaymentService(mockRepo)

		saveErr := errors.New("failed to save to database")
		mockRepo.On("GetByOrderID", ctx, "order-4").Return(repository.Transaction{}, repository.ErrNotFound).Once()
		mockRepo.On("Save", ctx, mock.MatchedBy(func(tx repository.Transaction) bool {
			return tx.OrderID == "order-4"
		})).Return(saveErr).Once()

		// Act
		transactionID, success, err := service.ProcessPayment(ctx, "order-4", "user-4", 400.0, "card")

		// Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to save transaction")
		require.False(t, success)
		require.Empty(t, transactionID)
		mockRepo.AssertExpectations(t)
	})
}

