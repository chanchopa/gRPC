package grpc

import (
	"context"
	"time"

	pb "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"payment-service/internal/usecase"
)

const MetadataKeyCustomerEmail = "x-customer-email"

type PaymentGRPCServer struct {
	pb.UnimplementedPaymentServiceServer
	uc *usecase.PaymentUseCase
}

func NewPaymentGRPCServer(uc *usecase.PaymentUseCase) *PaymentGRPCServer {
	return &PaymentGRPCServer{uc: uc}
}

func (s *PaymentGRPCServer) ProcessPayment(ctx context.Context, req *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	if req.OrderId == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}

	customerEmail := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get(MetadataKeyCustomerEmail)
		if len(values) > 0 {
			customerEmail = values[0]
		}
	}

	input := usecase.AuthorizeInput{
		OrderID:       req.OrderId,
		Amount:        req.Amount,
		CustomerEmail: customerEmail,
	}

	payment, err := s.uc.AuthorizePayment(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authorize payment: %v", err)
	}

	return &pb.PaymentResponse{
		Id:            payment.ID,
		OrderId:       payment.OrderID,
		TransactionId: payment.TransactionID,
		Amount:        payment.Amount,
		Status:        payment.Status,
		CreatedAt:     timestamppb.New(time.Now().UTC()),
	}, nil
}
