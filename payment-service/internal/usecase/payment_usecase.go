package usecase

import (
	"context"
	"fmt"

	"payment-service/internal/domain"

	"github.com/google/uuid"
)

type PaymentUseCase struct {
	repo domain.PaymentRepository
}

func NewPaymentUseCase(repo domain.PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{repo: repo}
}

type AuthorizeInput struct {
	OrderID string
	Amount  int64
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

	return payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	payment, err := uc.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("payment not found for order %s: %w", orderID, err)
	}
	return payment, nil
}
