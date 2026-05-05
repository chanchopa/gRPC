package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/domain"
	"notification-service/internal/usecase"
)

type Config struct {
	URL           string
	ExchangeName  string
	RoutingKey    string
	QueueName     string
	DLXName       string
	DLQName       string
	MaxRetries    int
	PrefetchCount int
}

type RabbitMQConsumer struct {
	cfg          Config
	conn         *amqp.Connection
	channel      *amqp.Channel
	handler      *usecase.ProcessEventUseCase
	retryCounter *retryCounter
}

func NewRabbitMQConsumer(cfg Config, handler *usecase.ProcessEventUseCase) (*RabbitMQConsumer, error) {
	var conn *amqp.Connection
	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		conn, err = amqp.Dial(cfg.URL)
		if err == nil {
			break
		}
		log.Printf("[notification-service][broker] dial attempt %d failed: %v", attempt, err)
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

	if err := ch.ExchangeDeclare(cfg.DLXName, "direct", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare dlx: %w", err)
	}
	if _, err := ch.QueueDeclare(cfg.DLQName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare dlq: %w", err)
	}
	if err := ch.QueueBind(cfg.DLQName, cfg.RoutingKey, cfg.DLXName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind dlq: %w", err)
	}

	if err := ch.ExchangeDeclare(cfg.ExchangeName, "direct", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    cfg.DLXName,
		"x-dead-letter-routing-key": cfg.RoutingKey,
	}
	if _, err := ch.QueueDeclare(cfg.QueueName, true, false, false, false, args); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare main queue: %w", err)
	}
	if err := ch.QueueBind(cfg.QueueName, cfg.RoutingKey, cfg.ExchangeName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("bind main queue: %w", err)
	}

	if cfg.PrefetchCount <= 0 {
		cfg.PrefetchCount = 10
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if err := ch.Qos(cfg.PrefetchCount, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	log.Printf("[notification-service][broker] connected, queue=%s dlq=%s prefetch=%d max_retries=%d",
		cfg.QueueName, cfg.DLQName, cfg.PrefetchCount, cfg.MaxRetries)

	return &RabbitMQConsumer{
		cfg:          cfg,
		conn:         conn,
		channel:      ch,
		handler:      handler,
		retryCounter: newRetryCounter(),
	}, nil
}

func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	deliveries, err := c.channel.Consume(
		c.cfg.QueueName,
		"notification-service",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	log.Printf("[notification-service][broker] consuming queue=%s", c.cfg.QueueName)

	for {
		select {
		case <-ctx.Done():
			log.Println("[notification-service][broker] context cancelled, stopping consume loop")
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed")
			}
			c.handleDelivery(ctx, d)
		}
	}
}

func (c *RabbitMQConsumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var event domain.PaymentCompletedEvent
	if err := json.Unmarshal(d.Body, &event); err != nil {
		log.Printf("[notification-service][broker] invalid payload, sending to DLQ: %v", err)
		_ = d.Nack(false, false)
		return
	}

	err := c.handler.Handle(ctx, event)
	if err == nil {
		c.retryCounter.delete(event.MessageID)
		if ackErr := d.Ack(false); ackErr != nil {
			log.Printf("[notification-service][broker] ack failed: %v", ackErr)
		}
		return
	}

	if errors.Is(err, usecase.ErrPermanentFailure) {
		log.Printf("[notification-service][broker] permanent failure, routing to DLQ: %v", err)
		c.retryCounter.delete(event.MessageID)
		_ = d.Nack(false, false)
		return
	}

	attempts := c.retryCounter.increment(event.MessageID)
	if attempts >= c.cfg.MaxRetries {
		log.Printf("[notification-service][broker] reached max retries (%d), routing to DLQ message_id=%s err=%v",
			c.cfg.MaxRetries, event.MessageID, err)
		c.retryCounter.delete(event.MessageID)
		_ = d.Nack(false, false)
		return
	}

	log.Printf("[notification-service][broker] transient error (attempt %d/%d), requeueing: %v",
		attempts, c.cfg.MaxRetries, err)
	_ = d.Nack(false, true)
}

func (c *RabbitMQConsumer) Close() error {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

type retryCounter struct {
	mu sync.Mutex
	m  map[string]int
}

func newRetryCounter() *retryCounter {
	return &retryCounter{m: make(map[string]int)}
}

func (r *retryCounter) increment(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[id] = r.m[id] + 1
	return r.m[id]
}

func (r *retryCounter) delete(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, id)
}
