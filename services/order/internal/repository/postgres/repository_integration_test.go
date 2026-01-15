//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib" //для goose миграций

	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

func TestRepository_Integration(t *testing.T) {
	ctx := context.Background()

	// Поднимаем PostgreSQL контейнер через testcontainers
	postgresContainer, err := postgres.RunContainer(ctx, //поднимаем контейнер
		testcontainers.WithImage("postgres:15-alpine"), //используем образ postgres:15-alpine
		postgres.WithDatabase("orders"),                //используем базу данных orders
		postgres.WithUsername("order_user"),            //используем пользователя order_user
		postgres.WithPassword("order_password"),        //используем пароль order_password
	)
	require.NoError(t, err)
	defer func() {
		err := postgresContainer.Terminate(ctx) //останавливаем контейнер и удаляем его
		require.NoError(t, err)
	}()

	// Получаем DSN из контейнера
	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable") //ConnectionString(...) собирает правильный DSN (включая реальный порт, который может быть не 5432).
	require.NoError(t, err)

	// Открываем *sql.DB через pgx stdlib для goose миграций
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()

	// Ждём готовности БД через ping с retry
	var pingErr error
	for i := 0; i < 10; i++ {
		pingErr = db.PingContext(ctx)
		if pingErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, pingErr, "Failed to ping database after retries")

	// Вычисляем путь к migrations относительно текущего файла
	_, filename, _, ok := runtime.Caller(0) //получаем путь к текущему файлу
	require.True(t, ok, "Failed to get current file path")

	// Текущий файл: services/order/internal/repository/postgres/repository_integration_test.go
	// Нужно получить: services/order/migrations
	testDir := filepath.Dir(filename)                        //получаем директорию текущего файла
	repoDir := filepath.Dir(testDir)                         // internal/repository
	internalDir := filepath.Dir(repoDir)                     // internal //получаем директорию internal
	serviceDir := filepath.Dir(internalDir)                  // services/order //получаем директорию services/order
	migrationsDir := filepath.Join(serviceDir, "migrations") //получаем путь к директории migrations

	// Накатываем миграции через goose
	err = goose.UpContext(ctx, db, migrationsDir)
	require.NoError(t, err, "Failed to run migrations")

	// Создаём pgxpool для repository
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Создаём repository
	repo := NewRepository(pool)

	t.Run("Save and GetByID", func(t *testing.T) {
		order := repository.Order{
			ID:     "order-1",
			UserID: "user-1",
			Status: "paid",
			Items: []repository.OrderItem{
				{ProductID: "product-1", Quantity: 2},
			},
		}

		// Сохраняем заказ
		err := repo.Save(ctx, order)
		require.NoError(t, err)

		// Получаем заказ по ID
		got, err := repo.GetByID(ctx, "order-1")
		require.NoError(t, err)

		// Проверяем основные поля
		require.Equal(t, order.ID, got.ID)
		require.Equal(t, order.UserID, got.UserID)
		require.Equal(t, order.Status, got.Status)

		// Проверяем items
		require.Len(t, got.Items, 1)
		require.Equal(t, order.Items[0].ProductID, got.Items[0].ProductID)
		require.Equal(t, order.Items[0].Quantity, got.Items[0].Quantity)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "missing")
		require.Error(t, err)
		require.True(t, errors.Is(err, repository.ErrNotFound), "Expected ErrNotFound, got: %v", err)
	})
}
