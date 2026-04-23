package main

import (
	"log"
	"os"

	"payment-service/internal/app"
)

func main() {
	cfg := app.Config{
		DBConnStr: getEnv("PAYMENT_DB_DSN", "postgres://payment_user:1234@localhost:5432/payment_db?sslmode=disable"),
		HTTPPort:  getEnv("PAYMENT_HTTP_PORT", "8081"),
		GRPCPort:  getEnv("PAYMENT_GRPC_PORT", "9091"),
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[payment-service] failed to initialize: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("[payment-service] server error: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
