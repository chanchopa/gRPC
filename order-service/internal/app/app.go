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
	pborder "github.com/ArlanAidarov/ap2-generated/order"
	pbpayment "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"order-service/internal/repository"
	transportgrpc "order-service/internal/transport/grpc"
	transporthttp "order-service/internal/transport/http"
	"order-service/internal/usecase"
)

type Config struct {
	DBConnStr       string
	HTTPPort        string
	GRPCPort        string
	PaymentGRPCAddr string
}

type App struct {
	cfg        Config
	httpServer *http.Server
	grpcServer *grpc.Server
	grpcConn   *grpc.ClientConn
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
	log.Println("[order-service] connected to postgres")

	grpcConn, err := grpc.NewClient(
		cfg.PaymentGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create payment grpc client: %w", err)
	}
	log.Printf("[order-service] payment gRPC target set to %s", cfg.PaymentGRPCAddr)

	orderRepo := repository.NewPostgresOrderRepository(db)
	paymentClient := transportgrpc.NewPaymentGRPCClient(pbpayment.NewPaymentServiceClient(grpcConn))
	orderUC := usecase.NewOrderUseCase(orderRepo, paymentClient)

	router := gin.Default()
	httpHandler := transporthttp.NewOrderHandler(orderUC)
	httpHandler.RegisterRoutes(router)
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	grpcServer := grpc.NewServer()
	pborder.RegisterOrderServiceServer(grpcServer, transportgrpc.NewOrderGRPCServer(db))

	return &App{
		cfg:        cfg,
		httpServer: httpServer,
		grpcServer: grpcServer,
		grpcConn:   grpcConn,
		db:         db,
	}, nil
}

func pingDBWithRetry(db *sql.DB, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := db.Ping(); err != nil {
			lastErr = err
			log.Printf("[order-service] db ping attempt %d failed: %v", i, err)
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
		log.Printf("[order-service] gRPC streaming server listening on :%s", a.cfg.GRPCPort)
		if err := a.grpcServer.Serve(lis); err != nil {
			grpcErrCh <- err
			return
		}
		grpcErrCh <- nil
	}()

	httpErrCh := make(chan error, 1)
	go func() {
		log.Printf("[order-service] HTTP listening on :%s", a.cfg.HTTPPort)
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
	log.Println("[order-service] graceful shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[order-service] http shutdown error: %v", err)
	} else {
		log.Println("[order-service] http server stopped")
	}

	stopped := make(chan struct{})
	go func() {
		a.grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		log.Println("[order-service] grpc server stopped")
	case <-shutdownCtx.Done():
		a.grpcServer.Stop()
		log.Println("[order-service] grpc server force-stopped")
	}

	if a.grpcConn != nil {
		if err := a.grpcConn.Close(); err != nil {
			log.Printf("[order-service] grpc client close error: %v", err)
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			log.Printf("[order-service] db close error: %v", err)
		}
	}

	log.Println("[order-service] shutdown complete")
	return nil
}
