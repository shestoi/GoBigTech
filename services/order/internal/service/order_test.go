package service

import (
	"context"
	"errors"
	"testing"

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
	repoMocks "github.com/shestoi/GoBigTech/services/order/internal/repository/mocks"
	"github.com/shestoi/GoBigTech/services/order/internal/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrderService_CreateOrder(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                string
		input               CreateOrderInput
		inventoryErrors     map[string]error // productID -> error
		paymentTransactionID string
		paymentError        error
		repoError           error
		expectedError       bool
		errorContains       string
		validateOrder       func(t *testing.T, order repository.Order)
		expectPaymentCalled bool
		expectRepoSaveCalled bool
	}{
		{
			name: "success: all steps succeed with single item",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
				},
			},
			inventoryErrors:       map[string]error{"product-456": nil},
			paymentTransactionID: "txn-789",
			paymentError:         nil,
			repoError:            nil,
			expectedError:        false,
			expectPaymentCalled:  true,
			expectRepoSaveCalled: true,
			validateOrder: func(t *testing.T, order repository.Order) {
				require.Equal(t, "user-123", order.UserID)
				require.Equal(t, "paid", order.Status)
				require.Len(t, order.Items, 1)
				require.Equal(t, "product-456", order.Items[0].ProductID)
				require.Equal(t, int32(3), order.Items[0].Quantity)
			},
		},
		{
			name: "success: all steps succeed with multiple items",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
					{
						ProductID: "product-789",
						Quantity:  2,
					},
				},
			},
			inventoryErrors: map[string]error{
				"product-456": nil,
				"product-789": nil,
			},
			paymentTransactionID: "txn-789",
			paymentError:         nil,
			repoError:            nil,
			expectedError:        false,
			expectPaymentCalled:  true,
			expectRepoSaveCalled: true,
			validateOrder: func(t *testing.T, order repository.Order) {
				require.Equal(t, "user-123", order.UserID)
				require.Equal(t, "paid", order.Status)
				require.Len(t, order.Items, 2)
				require.Equal(t, "product-456", order.Items[0].ProductID)
				require.Equal(t, int32(3), order.Items[0].Quantity)
				require.Equal(t, "product-789", order.Items[1].ProductID)
				require.Equal(t, int32(2), order.Items[1].Quantity)
			},
		},
		{
			name: "error: empty items",
			input: CreateOrderInput{
				UserID: "user-123",
				Items:  []repository.OrderItem{},
			},
			inventoryErrors:       nil,
			paymentTransactionID: "",
			paymentError:         nil,
			repoError:            nil,
			expectedError:        true,
			errorContains:        "order must contain at least one item",
			expectPaymentCalled:  false,
			expectRepoSaveCalled: false,
		},
		{
			name: "error: inventory ReserveStock fails for first item",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
				},
			},
			inventoryErrors:       map[string]error{"product-456": errors.New("insufficient stock")},
			paymentTransactionID: "",
			paymentError:         nil,
			repoError:            nil,
			expectedError:        true,
			errorContains:        "inventory service error",
			expectPaymentCalled:  false,
			expectRepoSaveCalled: false,
		},
		{
			name: "error: inventory ReserveStock fails for second item",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
					{
						ProductID: "product-789",
						Quantity:  2,
					},
				},
			},
			inventoryErrors: map[string]error{
				"product-456": nil,
				"product-789": errors.New("insufficient stock"),
			},
			paymentTransactionID: "",
			paymentError:         nil,
			repoError:            nil,
			expectedError:        true,
			errorContains:        "inventory service error",
			expectPaymentCalled:  false,
			expectRepoSaveCalled: false,
		},
		{
			name: "error: payment ProcessPayment fails",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
				},
			},
			inventoryErrors:       map[string]error{"product-456": nil},
			paymentTransactionID: "",
			paymentError:         errors.New("payment declined"),
			repoError:            nil,
			expectedError:        true,
			errorContains:        "payment service error",
			expectPaymentCalled:  true,
			expectRepoSaveCalled: false,
		},
		{
			name: "error: repository Save fails",
			input: CreateOrderInput{
				UserID: "user-123",
				Items: []repository.OrderItem{
					{
						ProductID: "product-456",
						Quantity:  3,
					},
				},
			},
			inventoryErrors:       map[string]error{"product-456": nil},
			paymentTransactionID: "txn-789",
			paymentError:         nil,
			repoError:            errors.New("database error"),
			expectedError:        true,
			errorContains:        "failed to save order",
			expectPaymentCalled:  true,
			expectRepoSaveCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockInventory := mocks.NewInventoryClient(t)
			mockPayment := mocks.NewPaymentClient(t)
			mockRepo := repoMocks.NewOrderRepository(t)

			service := NewOrderService(mockInventory, mockPayment, mockRepo)

			// Настройка моков для inventory (для каждого item)
			if tt.inventoryErrors != nil {
				for _, item := range tt.input.Items {
					err := tt.inventoryErrors[item.ProductID]
					mockInventory.On("ReserveStock", ctx, item.ProductID, item.Quantity).
						Return(err).Once()
				}
			}

			if tt.expectPaymentCalled {
				mockPayment.On("ProcessPayment", ctx, "order-123", tt.input.UserID, 100.0, "card").
					Return(tt.paymentTransactionID, tt.paymentError).Once()
			} else {
				mockPayment.AssertNotCalled(t, "ProcessPayment")
			}

			if tt.expectRepoSaveCalled {
				mockRepo.On("Save", ctx, mock.MatchedBy(func(order repository.Order) bool {
					if tt.validateOrder != nil {
						tt.validateOrder(t, order)
					}
					// Проверяем, что Items совпадают
					if len(order.Items) != len(tt.input.Items) {
						return false
					}
					for i, expectedItem := range tt.input.Items {
						if order.Items[i].ProductID != expectedItem.ProductID ||
							order.Items[i].Quantity != expectedItem.Quantity {
							return false
						}
					}
					return order.UserID == tt.input.UserID &&
						order.Status == "paid"
				})).Return(tt.repoError).Once()
			} else {
				mockRepo.AssertNotCalled(t, "Save")
			}

			// Act
			result, err := service.CreateOrder(ctx, tt.input)

			// Assert
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotEmpty(t, result.OrderID)
				require.Equal(t, tt.input.UserID, result.UserID)
				require.Equal(t, "paid", result.Status)
				require.Equal(t, len(tt.input.Items), len(result.Items))
				for i, expectedItem := range tt.input.Items {
					require.Equal(t, expectedItem.ProductID, result.Items[i].ProductID)
					require.Equal(t, expectedItem.Quantity, result.Items[i].Quantity)
				}
			}

			mockInventory.AssertExpectations(t)
			mockPayment.AssertExpectations(t)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestOrderService_GetOrder(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		input         GetOrderInput
		repoOrder     repository.Order
		repoError     error
		expectedError bool
		errorContains string
		validateOutput func(t *testing.T, output *GetOrderOutput)
	}{
		{
			name: "success: order with items",
			input: GetOrderInput{
				OrderID: "order-123",
			},
			repoOrder: repository.Order{
				ID:     "order-123",
				UserID: "user-456",
				Status: "paid",
				Items: []repository.OrderItem{
					{
						ProductID: "product-789",
						Quantity:  5,
					},
				},
			},
			repoError:     nil,
			expectedError: false,
			validateOutput: func(t *testing.T, output *GetOrderOutput) {
				require.Equal(t, "order-123", output.OrderID)
				require.Equal(t, "user-456", output.UserID)
				require.Equal(t, "paid", output.Status)
				require.Len(t, output.Items, 1)
				require.Equal(t, "product-789", output.Items[0].ProductID)
				require.Equal(t, int32(5), output.Items[0].Quantity)
			},
		},
		{
			name: "success: order with multiple items",
			input: GetOrderInput{
				OrderID: "order-456",
			},
			repoOrder: repository.Order{
				ID:     "order-456",
				UserID: "user-789",
				Status: "paid",
				Items: []repository.OrderItem{
					{
						ProductID: "product-111",
						Quantity:  2,
					},
					{
						ProductID: "product-222",
						Quantity:  3,
					},
				},
			},
			repoError:     nil,
			expectedError: false,
			validateOutput: func(t *testing.T, output *GetOrderOutput) {
				require.Equal(t, "order-456", output.OrderID)
				require.Equal(t, "user-789", output.UserID)
				require.Equal(t, "paid", output.Status)
				require.Len(t, output.Items, 2)
				require.Equal(t, "product-111", output.Items[0].ProductID)
				require.Equal(t, int32(2), output.Items[0].Quantity)
				require.Equal(t, "product-222", output.Items[1].ProductID)
				require.Equal(t, int32(3), output.Items[1].Quantity)
			},
		},
		{
			name: "error: order not found",
			input: GetOrderInput{
				OrderID: "order-999",
			},
			repoOrder:     repository.Order{},
			repoError:     repository.ErrNotFound,
			expectedError: true,
			errorContains: "failed to get order",
		},
		{
			name: "success: order without items",
			input: GetOrderInput{
				OrderID: "order-456",
			},
			repoOrder: repository.Order{
				ID:     "order-456",
				UserID: "user-789",
				Status: "pending",
				Items:  []repository.OrderItem{},
			},
			repoError:     nil,
			expectedError: false,
			validateOutput: func(t *testing.T, output *GetOrderOutput) {
				require.Equal(t, "order-456", output.OrderID)
				require.Equal(t, "user-789", output.UserID)
				require.Equal(t, "pending", output.Status)
				require.Len(t, output.Items, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockInventory := mocks.NewInventoryClient(t)
			mockPayment := mocks.NewPaymentClient(t)
			mockRepo := repoMocks.NewOrderRepository(t)

			service := NewOrderService(mockInventory, mockPayment, mockRepo)

			mockRepo.On("GetByID", ctx, tt.input.OrderID).
				Return(tt.repoOrder, tt.repoError).Once()

			// Act
			result, err := service.GetOrder(ctx, tt.input)

			// Assert
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validateOutput != nil {
					tt.validateOutput(t, result)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
