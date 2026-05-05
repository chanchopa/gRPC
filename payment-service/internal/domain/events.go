package domain

import "context"

type PaymentCompletedEvent struct {
	MessageID     string `json:"message_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

type EventPublisher interface {
	PublishPaymentCompleted(ctx context.Context, event PaymentCompletedEvent) error
	Close() error
}
