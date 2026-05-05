package domain

import "context"

type PaymentCompletedEvent struct {
	MessageID     string `json:"message_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

type IdempotencyStore interface {
	WasProcessed(ctx context.Context, messageID string) (bool, error)
	MarkProcessed(ctx context.Context, messageID, orderID string) error
}

type Notifier interface {
	Notify(ctx context.Context, event PaymentCompletedEvent) error
}
