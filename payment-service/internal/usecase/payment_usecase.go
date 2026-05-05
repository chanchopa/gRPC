package usecase

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"payment-service/internal/domain"
)

type PaymentUseCase struct {
	repo      domain.PaymentRepository
	publisher domain.EventPublisher
}

func NewPaymentUseCase(repo domain.PaymentRepository, publisher domain.EventPublisher) *PaymentUseCase {
	return &PaymentUseCase{repo: repo, publisher: publisher}
}

type AuthorizeInput struct {
	OrderID       string
	Amount        int64
	CustomerEmail string
}

func (uc *PaymentUseCase) AuthorizePayment(ctx context.Context, input AuthorizeInput) (*domain.Payment, error) {
	payment := &domain.Payment{
		ID:      uuid.NewString(),
		OrderID: input.OrderID,
		Amount:  input.Amount,
	}

	if err := payment.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if payment.ShouldDecline() {
		payment.TransactionID = ""
		payment.Status = domain.StatusDeclined
	} else {
		payment.TransactionID = uuid.NewString()
		payment.Status = domain.StatusAuthorized
	}

	if err := uc.repo.Save(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	if uc.publisher != nil {
		event := domain.PaymentCompletedEvent{
			MessageID:     uuid.NewString(),
			OrderID:       payment.OrderID,
			Amount:        payment.Amount,
			CustomerEmail: input.CustomerEmail,
			Status:        payment.Status,
		}
		if err := uc.publisher.PublishPaymentCompleted(ctx, event); err != nil {
			log.Printf("[payment-service] WARN failed to publish event for order %s: %v",
				payment.OrderID, err)
		}
	}

	return payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	payment, err := uc.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("payment not found for order %s: %w", orderID, err)
	}
	return payment, nil
}
