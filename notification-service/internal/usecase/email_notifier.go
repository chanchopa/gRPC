package usecase

import (
	"context"
	"fmt"
	"log"

	"notification-service/internal/domain"
)

type EmailNotifier struct{}

func NewEmailNotifier() *EmailNotifier {
	return &EmailNotifier{}
}

func (n *EmailNotifier) Notify(ctx context.Context, event domain.PaymentCompletedEvent) error {
	if event.CustomerEmail == "" {
		return fmt.Errorf("customer_email is empty for order %s", event.OrderID)
	}
	amount := float64(event.Amount) / 100.0
	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: $%.2f",
		event.CustomerEmail, event.OrderID, amount)
	return nil
}
