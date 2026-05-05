package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment-service/internal/domain"
)

type RabbitMQPublisher struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	routingKey   string
	confirms     chan amqp.Confirmation
}

type Config struct {
	URL          string
	ExchangeName string
	RoutingKey   string
	QueueName    string
	DLXName      string
	DLQName      string
}

func NewRabbitMQPublisher(cfg Config) (*RabbitMQPublisher, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		conn, err = amqp.Dial(cfg.URL)
		if err == nil {
			break
		}
		log.Printf("[payment-service][broker] dial attempt %d failed: %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.DLXName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare dlx: %w", err)
	}

	if _, err := ch.QueueDeclare(
		cfg.DLQName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare dlq: %w", err)
	}

	if err := ch.QueueBind(cfg.DLQName, cfg.RoutingKey, cfg.DLXName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind dlq: %w", err)
	}

	if err := ch.ExchangeDeclare(
		cfg.ExchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    cfg.DLXName,
		"x-dead-letter-routing-key": cfg.RoutingKey,
	}
	if _, err := ch.QueueDeclare(
		cfg.QueueName,
		true,
		false,
		false,
		false,
		args,
	); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare main queue: %w", err)
	}

	if err := ch.QueueBind(cfg.QueueName, cfg.RoutingKey, cfg.ExchangeName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind main queue: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	log.Printf("[payment-service][broker] connected, exchange=%s queue=%s dlq=%s",
		cfg.ExchangeName, cfg.QueueName, cfg.DLQName)

	return &RabbitMQPublisher{
		conn:         conn,
		channel:      ch,
		exchangeName: cfg.ExchangeName,
		routingKey:   cfg.RoutingKey,
		confirms:     confirms,
	}, nil
}

func (p *RabbitMQPublisher) PublishPaymentCompleted(ctx context.Context, event domain.PaymentCompletedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = p.channel.PublishWithContext(
		publishCtx,
		p.exchangeName,
		p.routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    event.MessageID,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	select {
	case confirm, ok := <-p.confirms:
		if !ok {
			return fmt.Errorf("confirm channel closed")
		}
		if !confirm.Ack {
			return fmt.Errorf("broker nacked message id=%s", event.MessageID)
		}
		log.Printf("[payment-service][broker] published payment.completed message_id=%s order_id=%s",
			event.MessageID, event.OrderID)
		return nil
	case <-publishCtx.Done():
		return fmt.Errorf("publish confirm timeout: %w", publishCtx.Err())
	}
}

func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
