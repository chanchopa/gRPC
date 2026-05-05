package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"notification-service/internal/broker"
	"notification-service/internal/repository"
	"notification-service/internal/usecase"
)

type Config struct {
	DBConnStr     string
	HTTPPort      string
	RabbitMQURL   string
	ExchangeName  string
	RoutingKey    string
	QueueName     string
	DLXName       string
	DLQName       string
	MaxRetries    int
	PrefetchCount int
}

type App struct {
	cfg        Config
	httpServer *http.Server
	consumer   *broker.RabbitMQConsumer
	db         *sql.DB
}

func New(cfg Config) (*App, error) {
	db, err := sql.Open("postgres", cfg.DBConnStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := pingDBWithRetry(db, 10, 2*time.Second); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	log.Println("[notification-service] connected to postgres")

	store := repository.NewPostgresIdempotencyStore(db)
	notifier := usecase.NewEmailNotifier()
	processor := usecase.NewProcessEventUseCase(store, notifier)

	consumer, err := broker.NewRabbitMQConsumer(broker.Config{
		URL:           cfg.RabbitMQURL,
		ExchangeName:  cfg.ExchangeName,
		RoutingKey:    cfg.RoutingKey,
		QueueName:     cfg.QueueName,
		DLXName:       cfg.DLXName,
		DLQName:       cfg.DLQName,
		MaxRetries:    cfg.MaxRetries,
		PrefetchCount: cfg.PrefetchCount,
	}, processor)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init consumer: %w", err)
	}

	router := gin.Default()
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &App{
		cfg:        cfg,
		httpServer: httpServer,
		consumer:   consumer,
		db:         db,
	}, nil
}

func pingDBWithRetry(db *sql.DB, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := db.Ping(); err != nil {
			lastErr = err
			log.Printf("[notification-service] db ping attempt %d failed: %v", i, err)
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return lastErr
}

func (a *App) Run(ctx context.Context) error {
	consumerErrCh := make(chan error, 1)
	go func() {
		consumerErrCh <- a.consumer.Start(ctx)
	}()

	httpErrCh := make(chan error, 1)
	go func() {
		log.Printf("[notification-service] HTTP listening on :%s", a.cfg.HTTPPort)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrCh <- err
			return
		}
		httpErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		return a.shutdown()
	case err := <-consumerErrCh:
		_ = a.shutdown()
		if err != nil {
			return fmt.Errorf("consumer: %w", err)
		}
		return nil
	case err := <-httpErrCh:
		_ = a.shutdown()
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	}
}

func (a *App) shutdown() error {
	log.Println("[notification-service] graceful shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[notification-service] http shutdown error: %v", err)
	} else {
		log.Println("[notification-service] http server stopped")
	}

	if a.consumer != nil {
		if err := a.consumer.Close(); err != nil {
			log.Printf("[notification-service] consumer close error: %v", err)
		} else {
			log.Println("[notification-service] consumer closed")
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Printf("[notification-service] db close error: %v", err)
		}
	}

	log.Println("[notification-service] shutdown complete")
	return nil
}
