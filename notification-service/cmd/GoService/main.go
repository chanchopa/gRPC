package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"notification-service/internal/app"
)

func main() {
	cfg := app.Config{
		DBConnStr:     getEnv("NOTIFICATION_DB_DSN", "postgres://notification_user:1234@localhost:5432/notification_db?sslmode=disable"),
		HTTPPort:      getEnv("NOTIFICATION_HTTP_PORT", "8082"),
		RabbitMQURL:   getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		ExchangeName:  getEnv("PAYMENT_EXCHANGE", "payments.exchange"),
		RoutingKey:    getEnv("PAYMENT_ROUTING_KEY", "payment.completed"),
		QueueName:     getEnv("PAYMENT_QUEUE", "payment.completed"),
		DLXName:       getEnv("PAYMENT_DLX", "payments.dlx"),
		DLQName:       getEnv("PAYMENT_DLQ", "payment.completed.dlq"),
		MaxRetries:    getEnvInt("MAX_RETRIES", 3),
		PrefetchCount: getEnvInt("PREFETCH_COUNT", 10),
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[notification-service] failed to initialize: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("[notification-service] runtime error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}
