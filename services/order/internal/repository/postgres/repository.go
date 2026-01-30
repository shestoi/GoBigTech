package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shestoi/GoBigTech/services/order/internal/repository"
)

// Repository реализует OrderRepository используя PostgreSQL
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository создаёт новый PostgreSQL репозиторий
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// Save сохраняет заказ в PostgreSQL
// Использует транзакцию для атомарного сохранения order и order_items
func (r *Repository) Save(ctx context.Context, order repository.Order) error {
	// Начинаем транзакцию
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Гарантируем откат транзакции в случае ошибки
	defer tx.Rollback(ctx)

	// Сохраняем order
	// Если CreatedAt == 0, используем DEFAULT now() из БД
	var createdAt time.Time
	if order.CreatedAt > 0 {
		createdAt = time.Unix(order.CreatedAt, 0)
		_, err = tx.Exec(ctx,
			`INSERT INTO orders (id, user_id, status, created_at) 
			 VALUES ($1, $2, $3, $4) 
			 ON CONFLICT (id) DO UPDATE SET 
			   user_id = EXCLUDED.user_id,
			   status = EXCLUDED.status,
			   created_at = EXCLUDED.created_at`,
			order.ID, order.UserID, order.Status, createdAt)
	} else {
		// Используем DEFAULT now() из БД
		_, err = tx.Exec(ctx,
			`INSERT INTO orders (id, user_id, status) 
			 VALUES ($1, $2, $3) 
			 ON CONFLICT (id) DO UPDATE SET 
			   user_id = EXCLUDED.user_id,
			   status = EXCLUDED.status`,
			order.ID, order.UserID, order.Status)
	}
	if err != nil {
		return err
	}

	// Удаляем старые items перед вставкой новых
	_, err = tx.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, order.ID)
	if err != nil {
		return err
	}

	//Даже если “в норме” order.ID уникальный, в коде репозитория часто делают Save() идемпотентным/обновляющим

	// Сохраняем order_items
	for _, item := range order.Items {
		_, err = tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity) 
			 VALUES ($1, $2, $3)`,
			order.ID, item.ProductID, item.Quantity)
		if err != nil {
			return err
		}
	}

	// Коммитим транзакцию
	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

// GetByID получает заказ по ID из PostgreSQL
// Собирает order и order_items в доменную модель
func (r *Repository) GetByID(ctx context.Context, id string) (repository.Order, error) {
	// Получаем order
	var order repository.Order
	var createdAt time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, status, created_at 
		 FROM orders 
		 WHERE id = $1`,
		id).Scan(&order.ID, &order.UserID, &order.Status, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.Order{}, repository.ErrNotFound
		}
		return repository.Order{}, err
	}

	// Конвертируем время в Unix timestamp
	order.CreatedAt = createdAt.Unix()

	// Получаем order_items
	rows, err := r.pool.Query(ctx,
		`SELECT product_id, quantity 
		 FROM order_items 
		 WHERE order_id = $1 
		 ORDER BY product_id`,
		id)
	if err != nil {
		return repository.Order{}, err
	}
	defer rows.Close()

	// Собираем items
	order.Items = make([]repository.OrderItem, 0)
	for rows.Next() {
		var item repository.OrderItem
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return repository.Order{}, err
		}
		order.Items = append(order.Items, item)
	}

	if err = rows.Err(); err != nil {
		return repository.Order{}, err
	}

	return order, nil
}

// HandleAssemblyCompletedTx обрабатывает событие завершения сборки заказа в транзакции
// Возвращает (inserted, rowsAffected, error):
//   - inserted=true если событие впервые обработано (вставлено в inbox)
//   - inserted=false если событие уже было обработано (duplicate event_id)
//   - rowsAffected - количество обновлённых строк в orders (0 или 1)
func (r *Repository) HandleAssemblyCompletedTx(ctx context.Context, eventID, eventType string, occurredAt time.Time, orderID string) (inserted bool, rowsAffected int64, err error) {
	// Начинаем транзакцию
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)

	// Пытаемся вставить событие в inbox
	_, err = tx.Exec(ctx,
		`INSERT INTO order_inbox_events (event_id, event_type, occurred_at, order_id)
		 VALUES ($1, $2, $3, $4)`,
		eventID, eventType, occurredAt, orderID)

	if err != nil {
		// Проверяем, это duplicate key error?
		var pgErr *pgconn.PgError                            //pgErr для проверки ошибки
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			// Событие уже обработано
			return false, 0, nil
		}
		// Другая ошибка
		return false, 0, err
	}

	// Событие впервые обработано
	inserted = true

	// Обновляем статус заказа: paid -> assembled
	result, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'assembled' 
		 WHERE id = $1 AND status = 'paid'`,
		orderID)
	if err != nil {
		return false, 0, err
	}

	rowsAffected = result.RowsAffected() //получаем количество обновлённых строк

	// Коммитим транзакцию
	if err = tx.Commit(ctx); err != nil {
		return false, 0, err
	}

	return inserted, rowsAffected, nil
}

// SaveWithOutbox сохраняет заказ и добавляет событие в outbox в одной транзакции
func (r *Repository) SaveWithOutbox(ctx context.Context, order repository.Order, eventID, eventType string, occurredAt time.Time, payload []byte, topic string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Сохраняем order
	var createdAt time.Time
	if order.CreatedAt > 0 {
		createdAt = time.Unix(order.CreatedAt, 0)
		_, err = tx.Exec(ctx,
			`INSERT INTO orders (id, user_id, status, created_at) 
			 VALUES ($1, $2, $3, $4) 
			 ON CONFLICT (id) DO UPDATE SET 
			   user_id = EXCLUDED.user_id,
			   status = EXCLUDED.status,
			   created_at = EXCLUDED.created_at`,
			order.ID, order.UserID, order.Status, createdAt)
	} else {
		_, err = tx.Exec(ctx,
			`INSERT INTO orders (id, user_id, status) 
			 VALUES ($1, $2, $3) 
			 ON CONFLICT (id) DO UPDATE SET 
			   user_id = EXCLUDED.user_id,
			   status = EXCLUDED.status`,
			order.ID, order.UserID, order.Status)
	}
	if err != nil {
		return err
	}

	// Удаляем старые items
	_, err = tx.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, order.ID)
	if err != nil {
		return err
	}

	// Сохраняем order_items
	for _, item := range order.Items {
		_, err = tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity) 
			 VALUES ($1, $2, $3)`,
			order.ID, item.ProductID, item.Quantity)
		if err != nil {
			return err
		}
	}

	// Добавляем событие в outbox
	_, err = tx.Exec(ctx,
		`INSERT INTO order_outbox_events (event_id, event_type, occurred_at, aggregate_id, payload, topic, status)
		 VALUES ($1, $2, $3, $4, $5, $6, 'pending')`,
		eventID, eventType, occurredAt, order.ID, payload, topic)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetPendingOutboxEvents получает pending события из outbox для отправки
// pending - это статус события, которое нужно отправить
func (r *Repository) GetPendingOutboxEvents(ctx context.Context, limit int) ([]repository.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT event_id, event_type, occurred_at, aggregate_id, payload, topic, status, attempts, last_error, created_at, sent_at
		 FROM order_outbox_events
		 WHERE status = 'pending' 
		 ORDER BY created_at ASC
		 LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]repository.OutboxEvent, 0)
	for rows.Next() {
		var event repository.OutboxEvent
		var sentAt *time.Time
		err := rows.Scan(
			&event.EventID, &event.EventType, &event.OccurredAt, &event.AggregateID,
			&event.Payload, &event.Topic, &event.Status, &event.Attempts,
			&event.LastError, &event.CreatedAt, &sentAt)
		if err != nil {
			return nil, err
		}
		if sentAt != nil {
			event.SentAt = *sentAt
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// MarkOutboxEventSent отмечает событие как отправленное
func (r *Repository) MarkOutboxEventSent(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE order_outbox_events 
		 SET status = 'sent', sent_at = NOW()
		 WHERE event_id = $1`,
		eventID)
	return err
}

// MarkOutboxEventFailed отмечает событие как failed и увеличивает attempts
func (r *Repository) MarkOutboxEventFailed(ctx context.Context, eventID string, errMsg string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE order_outbox_events 
		 SET status = 'failed', attempts = attempts + 1, last_error = $2
		 WHERE event_id = $1`,
		eventID, errMsg)
	return err
}

// ResetOutboxEventPending сбрасывает статус события на pending для retry
func (r *Repository) ResetOutboxEventPending(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE order_outbox_events 
		 SET status = 'pending'
		 WHERE event_id = $1`,
		eventID)
	return err
}

//package postgres
//
//import (
//	"context"
//	"errors"
//	"time"
//
//	sq "github.com/Masterminds/squirrel"
//	"github.com/jackc/pgx/v5"
//	"github.com/jackc/pgx/v5/pgxpool"
//
//	"github.com/shestoi/GoBigTech/services/order/internal/repository"
//)
//
//type Repository struct {
//	pool *pgxpool.Pool
//}
//
//func NewRepository(pool *pgxpool.Pool) *Repository {
//	return &Repository{pool: pool}
//}
//
//// GetByID получает заказ и его items по ID.
//// Тут squirrel нужен не потому что "без него нельзя",
//// а чтобы показать подход: builder -> SQL string + args -> pgx Query/QueryRow.
//func (r *Repository) GetByID(ctx context.Context, id string) (repository.Order, error) {
//	// Squirrel builder нужно настроить на плейсхолдеры PostgreSQL: $1, $2, ...
//	// Иначе по умолчанию будет "?" (MySQL-style), и pgx не поймёт.
//	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
//
//	// ---------------------------
//	// 1) Забираем сам заказ (orders)
//	// ---------------------------
//	orderSQL, orderArgs, err := psql.
//		Select("id", "user_id", "status", "created_at").
//		From("orders").
//		Where(sq.Eq{"id": id}). // WHERE id = $1
//		Limit(1).
//		ToSql()
//	if err != nil {
//		return repository.Order{}, err
//	}
//
//	var order repository.Order
//	var createdAt time.Time
//
//	// QueryRow — потому что ожидаем ровно одну строку
//	err = r.pool.QueryRow(ctx, orderSQL, orderArgs...).
//		Scan(&order.ID, &order.UserID, &order.Status, &createdAt)
//
//	if err != nil {
//		// pgx.ErrNoRows = "ничего не нашлось"
//		if errors.Is(err, pgx.ErrNoRows) {
//			return repository.Order{}, repository.ErrNotFound
//		}
//		return repository.Order{}, err
//	}
//
//	// Конвертация TIMESTAMPTZ -> Unix timestamp (как у тебя в доменной модели)
//	order.CreatedAt = createdAt.Unix()
//
//	// ---------------------------
//	// 2) Забираем items (order_items)
//	// ---------------------------
//	itemsSQL, itemsArgs, err := psql.
//		Select("product_id", "quantity").
//		From("order_items").
//		Where(sq.Eq{"order_id": id}). // WHERE order_id = $1
//		OrderBy("product_id ASC").    // просто чтобы порядок был стабильным
//		ToSql()
//	if err != nil {
//		return repository.Order{}, err
//	}
//
//	rows, err := r.pool.Query(ctx, itemsSQL, itemsArgs...)
//	if err != nil {
//		return repository.Order{}, err
//	}
//	defer rows.Close()
//
//	// make([]T, 0) — создаём пустой slice (не nil).
//	// Это удобно: при отсутствии items вернём [] а не nil (часто легче для клиента/логики).
//	order.Items = make([]repository.OrderItem, 0)
//
//	for rows.Next() {
//		var it repository.OrderItem
//		if err := rows.Scan(&it.ProductID, &it.Quantity); err != nil {
//			return repository.Order{}, err
//		}
//		order.Items = append(order.Items, it)
//	}
//
//	// Если при обходе строк была ошибка
//	if err := rows.Err(); err != nil {
//		return repository.Order{}, err
//	}
//
//	return order, nil
//}
