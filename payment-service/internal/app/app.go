package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	pb "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc"

	"payment-service/internal/broker"
	"payment-service/internal/repository"
	transportgrpc "payment-service/internal/transport/grpc"
	transporthttp "payment-service/internal/transport/http"
	"payment-service/internal/usecase"
)

type Config struct {
	DBConnStr        string
	HTTPPort         string
	GRPCPort         string
	RabbitMQURL      string
	ExchangeName     string
	RoutingKey       string
	QueueName        string
	DLXName          string
	DLQName          string
}

type App struct {
	cfg        Config
	httpServer *http.Server
	grpcServer *grpc.Server
	publisher  *broker.RabbitMQPublisher
	db         *sql.DB
}

func New(cfg Config) (*App, error) {
	db, err := sql.Open("postgres", cfg.DBConnStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := pingDBWithRetry(db, 10, 2*time.Second); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	log.Println("[payment-service] connected to postgres")

	publisher, err := broker.NewRabbitMQPublisher(broker.Config{
		URL:          cfg.RabbitMQURL,
		ExchangeName: cfg.ExchangeName,
		RoutingKey:   cfg.RoutingKey,
		QueueName:    cfg.QueueName,
		DLXName:      cfg.DLXName,
		DLQName:      cfg.DLQName,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init publisher: %w", err)
	}

	paymentRepo := repository.NewPostgresPaymentRepository(db)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo, publisher)

	router := gin.Default()
	httpHandler := transporthttp.NewPaymentHandler(paymentUC)
	httpHandler.RegisterRoutes(router)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(transportgrpc.LoggingInterceptor),
	)
	pb.RegisterPaymentServiceServer(grpcServer, transportgrpc.NewPaymentGRPCServer(paymentUC))

	return &App{
		cfg:        cfg,
		httpServer: httpServer,
		grpcServer: grpcServer,
		publisher:  publisher,
		db:         db,
	}, nil
}

func pingDBWithRetry(db *sql.DB, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := db.Ping(); err != nil {
			lastErr = err
			log.Printf("[payment-service] db ping attempt %d failed: %v", i, err)
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return lastErr
}

func (a *App) Run(ctx context.Context) error {
	grpcErrCh := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", ":"+a.cfg.GRPCPort)
		if err != nil {
			grpcErrCh <- fmt.Errorf("grpc listen: %w", err)
			return
		}
		log.Printf("[payment-service] gRPC listening on :%s", a.cfg.GRPCPort)
		if err := a.grpcServer.Serve(lis); err != nil {
			grpcErrCh <- err
			return
		}
		grpcErrCh <- nil
	}()

	httpErrCh := make(chan error, 1)
	go func() {
		log.Printf("[payment-service] HTTP listening on :%s", a.cfg.HTTPPort)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrCh <- err
			return
		}
		httpErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		return a.shutdown()
	case err := <-grpcErrCh:
		_ = a.shutdown()
		if err != nil {
			return fmt.Errorf("grpc server: %w", err)
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
	log.Println("[payment-service] graceful shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[payment-service] http shutdown error: %v", err)
	} else {
		log.Println("[payment-service] http server stopped")
	}

	stopped := make(chan struct{})
	go func() {
		a.grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		log.Println("[payment-service] grpc server stopped")
	case <-shutdownCtx.Done():
		a.grpcServer.Stop()
		log.Println("[payment-service] grpc server force-stopped")
	}

	if a.publisher != nil {
		if err := a.publisher.Close(); err != nil {
			log.Printf("[payment-service] publisher close error: %v", err)
		} else {
			log.Println("[payment-service] publisher closed")
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Printf("[payment-service] db close error: %v", err)
		} else {
			log.Println("[payment-service] db closed")
		}
	}

	log.Println("[payment-service] shutdown complete")
	return nil
}
