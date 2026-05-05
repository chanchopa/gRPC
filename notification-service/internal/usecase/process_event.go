package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"

	"notification-service/internal/domain"
)

var ErrPermanentFailure = errors.New("permanent processing failure")

type ProcessEventUseCase struct {
	store    domain.IdempotencyStore
	notifier domain.Notifier
}

func NewProcessEventUseCase(store domain.IdempotencyStore, notifier domain.Notifier) *ProcessEventUseCase {
	return &ProcessEventUseCase{store: store, notifier: notifier}
}

func (uc *ProcessEventUseCase) Handle(ctx context.Context, event domain.PaymentCompletedEvent) error {
	if event.MessageID == "" {
		return fmt.Errorf("%w: missing message_id", ErrPermanentFailure)
	}
	if event.OrderID == "" {
		return fmt.Errorf("%w: missing order_id", ErrPermanentFailure)
	}

	already, err := uc.store.WasProcessed(ctx, event.MessageID)
	if err != nil {
		return fmt.Errorf("idempotency lookup: %w", err)
	}
	if already {
		log.Printf("[notification-service] duplicate event ignored message_id=%s order_id=%s",
			event.MessageID, event.OrderID)
		return nil
	}

	if err := uc.notifier.Notify(ctx, event); err != nil {
		return fmt.Errorf("notify: %w", err)
	}

	if err := uc.store.MarkProcessed(ctx, event.MessageID, event.OrderID); err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}

	return nil
}
