package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shestoi/GoBigTech/services/payment/internal/repository"
)

// PaymentService содержит бизнес-логику работы с платежами
// Использует только простые типы Go, не зависит от protobuf
// Зависит от интерфейса PaymentRepository, а не от конкретной реализации
type PaymentService struct {
	repo repository.PaymentRepository
}

// NewPaymentService создаёт новый экземпляр PaymentService
// Принимает repository как зависимость - это позволяет легко подменять его в тестах
func NewPaymentService(repo repository.PaymentRepository) *PaymentService {
	return &PaymentService{
		repo: repo,
	}
}

// ProcessPayment обрабатывает платеж
// Реализует идемпотентность: повторный вызов для того же orderID возвращает тот же transactionID
// Возвращает transaction ID, success и ошибку
func (s *PaymentService) ProcessPayment(ctx context.Context, orderID, userID string, amount float64, method string) (transactionID string, success bool, err error) {
	log.Printf("ProcessPayment called: order=%s, user=%s, amount=%f, method=%s",
		orderID, userID, amount, method)

	// a) Валидация: сумма должна быть положительной
	if amount <= 0 {
		return "", false, fmt.Errorf("invalid amount: must be greater than 0")
	}

	// b) Проверяем, существует ли уже транзакция для этого orderID (идемпотентность)
	existingTx, err := s.repo.GetByOrderID(ctx, orderID)
	if err == nil {
		// Транзакция найдена - возвращаем существующий transactionID (идемпотентность)
		log.Printf("Payment already processed for order=%s, returning existing transactionID=%s",
			orderID, existingTx.TransactionID)
		return existingTx.TransactionID, true, nil
	}

	// Если ошибка не ErrNotFound, возвращаем её
	if err != repository.ErrNotFound {
		log.Printf("Error getting transaction: %v", err)
		return "", false, fmt.Errorf("failed to check existing transaction: %w", err)
	}

	// c) Транзакция не найдена - создаём новую
	// Генерируем transaction ID: tx_{orderID}_{timestamp}
	transactionID = fmt.Sprintf("tx_%s_%d", orderID, time.Now().Unix())

	// Создаём доменную модель транзакции
	tx := repository.Transaction{
		OrderID:       orderID,
		UserID:        userID,
		Amount:        amount,
		Method:        method,
		TransactionID: transactionID,
		Status:        "success",
		CreatedAt:     time.Now().Unix(),
	}

	// Сохраняем транзакцию в repository
	if err := s.repo.Save(ctx, tx); err != nil {
		log.Printf("Failed to save transaction: %v", err)
		return "", false, fmt.Errorf("failed to save transaction: %w", err)
	}

	log.Printf("Payment processed successfully: transactionID=%s", transactionID)
	return transactionID, true, nil
}
