package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"payment-service/internal/domain"
)

type PostgresPaymentRepository struct {
	db *sql.DB
}

func NewPostgresPaymentRepository(db *sql.DB) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{db: db}
}

func (r *PostgresPaymentRepository) Save(ctx context.Context, payment *domain.Payment) error {
	query := `
		INSERT INTO payments (id, order_id, transaction_id, amount, status)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		payment.ID,
		payment.OrderID,
		payment.TransactionID,
		payment.Amount,
		payment.Status,
	)
	if err != nil {
		return fmt.Errorf("postgres save payment: %w", err)
	}
	return nil
}

func (r *PostgresPaymentRepository) FindByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, transaction_id, amount, status
		FROM payments WHERE order_id = $1
		ORDER BY created_at DESC LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, orderID)

	p := &domain.Payment{}
	var txID sql.NullString
	err := row.Scan(&p.ID, &p.OrderID, &txID, &p.Amount, &p.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("payment for order %s not found", orderID)
		}
		return nil, fmt.Errorf("postgres find payment: %w", err)
	}
	if txID.Valid {
		p.TransactionID = txID.String
	}
	return p, nil
}
