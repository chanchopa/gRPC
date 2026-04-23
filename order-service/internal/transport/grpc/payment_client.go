package grpc

import (
	"context"
	"fmt"

	pb "github.com/ArlanAidarov/ap2-generated/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"order-service/internal/domain"
)

type PaymentGRPCClient struct {
	client pb.PaymentServiceClient
}

func NewPaymentGRPCClient(client pb.PaymentServiceClient) *PaymentGRPCClient {
	return &PaymentGRPCClient{client: client}
}

func (c *PaymentGRPCClient) Authorize(ctx context.Context, orderID string, amount int64) (domain.PaymentResult, error) {
	req := &pb.PaymentRequest{
		OrderId: orderID,
		Amount:  amount,
	}

	resp, err := c.client.ProcessPayment(ctx, req)
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.DeadlineExceeded || st.Code() == codes.Unavailable {
			return domain.PaymentResult{}, fmt.Errorf("payment service unavailable: %w", err)
		}
		return domain.PaymentResult{}, fmt.Errorf("payment grpc call failed: %w", err)
	}

	return domain.PaymentResult{
		TransactionID: resp.GetTransactionId(),
		Status:        resp.GetStatus(),
	}, nil
}
