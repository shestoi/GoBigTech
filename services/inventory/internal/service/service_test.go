package service

import (
	"context"
	"errors"
	"testing"

	"github.com/shestoi/GoBigTech/services/inventory/internal/repository"
	"github.com/shestoi/GoBigTech/services/inventory/internal/repository/mocks"
	"github.com/stretchr/testify/require"
)

func TestInventoryService_GetStock(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		productID      string
		repoReturn     int32
		repoError      error
		expectedResult int32
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "success: returns available stock",
			productID:      "product-1",
			repoReturn:     10,
			repoError:      nil,
			expectedResult: 10,
			expectedError:  false,
		},
		{
			name:           "success: returns zero stock",
			productID:      "product-2",
			repoReturn:     0,
			repoError:      nil,
			expectedResult: 0,
			expectedError:  false,
		},
		{
			name:           "ErrNotFound returns default 42",
			productID:      "product-3",
			repoReturn:     0,
			repoError:      repository.ErrNotFound,
			expectedResult: 42,
			expectedError:  false,
		},
		{
			name:           "arbitrary error returns error",
			productID:      "product-4",
			repoReturn:     0,
			repoError:      errors.New("database connection failed"),
			expectedResult: 0,
			expectedError:  true,
			errorContains:  "database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := mocks.NewInventoryRepository(t)
			service := NewInventoryService(mockRepo)

			mockRepo.On("GetStock", ctx, tt.productID).Return(tt.repoReturn, tt.repoError).Once()

			// Act
			result, err := service.GetStock(ctx, tt.productID)

			// Assert
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, int32(0), result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestInventoryService_ReserveStock(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		productID      string
		quantity       int32
		repoReturn     bool
		repoError      error
		expectedResult bool
		expectedError  bool
		errorContains  string
	}{
		{
			name:           "success: reservation successful",
			productID:      "product-1",
			quantity:       5,
			repoReturn:     true,
			repoError:      nil,
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:           "success: insufficient stock returns false",
			productID:      "product-2",
			quantity:       100,
			repoReturn:     false,
			repoError:      nil,
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "error: repository returns error",
			productID:      "product-3",
			quantity:       10,
			repoReturn:     false,
			repoError:      errors.New("database connection failed"),
			expectedResult: false,
			expectedError:  true,
			errorContains:  "database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := mocks.NewInventoryRepository(t)
			service := NewInventoryService(mockRepo)

			mockRepo.On("ReserveStock", ctx, tt.productID, tt.quantity).Return(tt.repoReturn, tt.repoError).Once()

			// Act
			result, err := service.ReserveStock(ctx, tt.productID, tt.quantity)

			// Assert
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.False(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}


