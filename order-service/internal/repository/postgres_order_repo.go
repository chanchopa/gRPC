package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"order-service/internal/domain"
)

type PostgresOrderRepository struct {
	db *sql.DB
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

func (r *PostgresOrderRepository) Save(ctx context.Context, order *domain.Order) error {
	query := `
		INSERT INTO orders (id, customer_id, item_name, amount, status, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var ikey interface{}
	if order.IdempotencyKey != "" {
		ikey = order.IdempotencyKey
	} else {
		ikey = nil
	}

	_, err := r.db.ExecContext(ctx, query,
		order.ID,
		order.CustomerID,
		order.ItemName,
		order.Amount,
		order.Status,
		ikey,
		order.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres save order: %w", err)
	}
	return nil
}

func (r *PostgresOrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	query := `
		SELECT id, customer_id, item_name, amount, status, created_at
		FROM orders WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	order := &domain.Order{}
	var createdAt time.Time
	err := row.Scan(
		&order.ID,
		&order.CustomerID,
		&order.ItemName,
		&order.Amount,
		&order.Status,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("order %s not found", id)
		}
		return nil, fmt.Errorf("postgres find order: %w", err)
	}
	order.CreatedAt = createdAt
	return order, nil
}

func (r *PostgresOrderRepository) Update(ctx context.Context, order *domain.Order) error {
	query := `UPDATE orders SET status = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, order.Status, order.ID)
	if err != nil {
		return fmt.Errorf("postgres update order: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("order %s not found for update", order.ID)
	}
	return nil
}

func (r *PostgresOrderRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	if key == "" {
		return nil, errors.New("empty idempotency key")
	}
	query := `
		SELECT id, customer_id, item_name, amount, status, created_at
		FROM orders WHERE idempotency_key = $1
	`
	row := r.db.QueryRowContext(ctx, query, key)

	order := &domain.Order{}
	var createdAt time.Time
	err := row.Scan(
		&order.ID,
		&order.CustomerID,
		&order.ItemName,
		&order.Amount,
		&order.Status,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no order with idempotency key %s", key)
		}
		return nil, fmt.Errorf("postgres find by idempotency key: %w", err)
	}
	order.CreatedAt = createdAt
	return order, nil
}
