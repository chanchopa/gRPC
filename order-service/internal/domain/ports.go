package domain

import "context"

type OrderRepository interface {
	Save(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
	Update(ctx context.Context, order *Order) error
	FindByIdempotencyKey(ctx context.Context, key string) (*Order, error)
}

type PaymentClient interface {
	Authorize(ctx context.Context, orderID string, amount int64) (PaymentResult, error)
}

type PaymentResult struct {
	TransactionID string
	Status        string
}
