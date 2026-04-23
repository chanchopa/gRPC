package app

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc"

	"payment-service/internal/repository"
	transportgrpc "payment-service/internal/transport/grpc"
	transporthttp "payment-service/internal/transport/http"
	"payment-service/internal/usecase"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type Config struct {
	DBConnStr   string
	HTTPPort    string
	GRPCPort    string
}

type App struct {
	cfg        Config
	httpRouter *gin.Engine
	grpcServer *grpc.Server
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
	log.Println("[payment-service] connected to postgres")

	paymentRepo := repository.NewPostgresPaymentRepository(db)
	paymentUC := usecase.NewPaymentUseCase(paymentRepo)

	httpHandler := transporthttp.NewPaymentHandler(paymentUC)
	router := gin.Default()
	httpHandler.RegisterRoutes(router)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(transportgrpc.LoggingInterceptor),
	)
	pb.RegisterPaymentServiceServer(grpcServer, transportgrpc.NewPaymentGRPCServer(paymentUC))

	return &App{cfg: cfg, httpRouter: router, grpcServer: grpcServer}, nil
}

func (a *App) Run() error {
	grpcErrCh := make(chan error, 1)
	go func() {
		lis, err := net.Listen("tcp", ":"+a.cfg.GRPCPort)
		if err != nil {
			grpcErrCh <- fmt.Errorf("grpc listen: %w", err)
			return
		}
		log.Printf("[payment-service] gRPC listening on :%s", a.cfg.GRPCPort)
		grpcErrCh <- a.grpcServer.Serve(lis)
	}()

	httpErrCh := make(chan error, 1)
	go func() {
		log.Printf("[payment-service] HTTP listening on :%s", a.cfg.HTTPPort)
		httpErrCh <- a.httpRouter.Run(":" + a.cfg.HTTPPort)
	}()

	select {
	case err := <-grpcErrCh:
		return fmt.Errorf("grpc server: %w", err)
	case err := <-httpErrCh:
		return fmt.Errorf("http server: %w", err)
	}
}
