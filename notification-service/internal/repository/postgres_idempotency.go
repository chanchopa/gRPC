package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type PostgresIdempotencyStore struct {
	db *sql.DB
}

func NewPostgresIdempotencyStore(db *sql.DB) *PostgresIdempotencyStore {
	return &PostgresIdempotencyStore{db: db}
}

func (s *PostgresIdempotencyStore) WasProcessed(ctx context.Context, messageID string) (bool, error) {
	var found int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM processed_messages WHERE message_id = $1`, messageID,
	).Scan(&found)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query processed_messages: %w", err)
	}
	return true, nil
}

func (s *PostgresIdempotencyStore) MarkProcessed(ctx context.Context, messageID, orderID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO processed_messages (message_id, order_id) VALUES ($1, $2)`,
		messageID, orderID,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil
		}
		return fmt.Errorf("insert processed_messages: %w", err)
	}
	return nil
}
