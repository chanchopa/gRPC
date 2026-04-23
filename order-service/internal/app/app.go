package app

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	pborder "github.com/ArlanAidarov/ap2-generated/order"
	pbpayment "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"order-service/internal/repository"
	transportgrpc "order-service/internal/transport/grpc"
	transporthttp "order-service/internal/transport/http"
	"order-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type Config struct {
	DBConnStr       string
	HTTPPort        string
	GRPCPort        string
	PaymentGRPCAddr string
}

type App struct {
	cfg        Config
	httpRouter *gin.Engine
	grpcServer *grpc.Server
	grpcConn   *grpc.ClientConn
}

func New(cfg Config) (*App, error) {
	db, err := sql.Open("postgres", cfg.DBConnStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	log.Println("[order-service] connected to postgres")

	grpcConn, err := grpc.NewClient(
		cfg.PaymentGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create payment grpc client: %w", err)
	}
	log.Printf("[order-service] payment gRPC target set to %s", cfg.PaymentGRPCAddr)

	orderRepo := repository.NewPostgresOrderRepository(db)
	paymentClient := transportgrpc.NewPaymentGRPCClient(pbpayment.NewPaymentServiceClient(grpcConn))
	orderUC := usecase.NewOrderUseCase(orderRepo, paymentClient)

	httpHandler := transporthttp.NewOrderHandler(orderUC)
	router := gin.Default()
	httpHandler.RegisterRoutes(router)

	grpcServer := grpc.NewServer()
	pborder.RegisterOrderServiceServer(grpcServer, transportgrpc.NewOrderGRPCServer(db))

	return &App{cfg: cfg, httpRouter: router, grpcServer: grpcServer, grpcConn: grpcConn}, nil
}

func (a *App) Run() error {
	grpcErrCh := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", ":"+a.cfg.GRPCPort)
		if err != nil {
			grpcErrCh <- fmt.Errorf("grpc listen: %w", err)
			return
		}
		log.Printf("[order-service] gRPC streaming server listening on :%s", a.cfg.GRPCPort)
		grpcErrCh <- a.grpcServer.Serve(lis)
	}()

	httpErrCh := make(chan error, 1)
	go func() {
		log.Printf("[order-service] HTTP listening on :%s", a.cfg.HTTPPort)
		httpErrCh <- a.httpRouter.Run(":" + a.cfg.HTTPPort)
	}()

	select {
	case err := <-grpcErrCh:
		return fmt.Errorf("grpc server: %w", err)
	case err := <-httpErrCh:
		return fmt.Errorf("http server: %w", err)
	}
}
