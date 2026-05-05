package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"payment-service/internal/app"
)

func main() {
	cfg := app.Config{
		DBConnStr:    getEnv("PAYMENT_DB_DSN", "postgres://payment_user:1234@localhost:5432/payment_db?sslmode=disable"),
		HTTPPort:     getEnv("PAYMENT_HTTP_PORT", "8081"),
		GRPCPort:     getEnv("PAYMENT_GRPC_PORT", "9091"),
		RabbitMQURL:  getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		ExchangeName: getEnv("PAYMENT_EXCHANGE", "payments.exchange"),
		RoutingKey:   getEnv("PAYMENT_ROUTING_KEY", "payment.completed"),
		QueueName:    getEnv("PAYMENT_QUEUE", "payment.completed"),
		DLXName:      getEnv("PAYMENT_DLX", "payments.dlx"),
		DLQName:      getEnv("PAYMENT_DLQ", "payment.completed.dlq"),
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[payment-service] failed to initialize: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("[payment-service] server error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
