package domain

import "context"

type PaymentRepository interface {
	Save(ctx context.Context, payment *Payment) error
	FindByOrderID(ctx context.Context, orderID string) (*Payment, error)
}
