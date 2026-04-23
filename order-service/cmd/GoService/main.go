package main

import (
	"log"
	"os"

	"order-service/internal/app"
)

func main() {
	cfg := app.Config{
		DBConnStr:       getEnv("ORDER_DB_DSN", "postgres://order_user:1234@localhost:5432/order_db?sslmode=disable"),
		HTTPPort:        getEnv("ORDER_HTTP_PORT", "8080"),
		GRPCPort:        getEnv("ORDER_GRPC_PORT", "9090"),
		PaymentGRPCAddr: getEnv("PAYMENT_GRPC_ADDR", "localhost:9091"),
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[order-service] failed to initialize: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("[order-service] server error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
