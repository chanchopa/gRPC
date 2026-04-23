package grpc

import (
	"database/sql"
	"time"

	pb "github.com/ArlanAidarov/ap2-generated/order"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderGRPCServer struct {
	pb.UnimplementedOrderServiceServer
	db *sql.DB
}

func NewOrderGRPCServer(db *sql.DB) *OrderGRPCServer {
	return &OrderGRPCServer{db: db}
}

func (s *OrderGRPCServer) SubscribeToOrderUpdates(req *pb.OrderRequest, stream pb.OrderService_SubscribeToOrderUpdatesServer) error {
	if req.OrderId == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	lastStatus := ""

	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
		}

		var currentStatus string
		err := s.db.QueryRowContext(stream.Context(),
			"SELECT status FROM orders WHERE id = $1", req.OrderId,
		).Scan(&currentStatus)

		if err != nil {
			if err == sql.ErrNoRows {
				return status.Errorf(codes.NotFound, "order %s not found", req.OrderId)
			}
			return status.Errorf(codes.Internal, "db query: %v", err)
		}

		if currentStatus != lastStatus {
			update := &pb.OrderStatusUpdate{
				OrderId:   req.OrderId,
				Status:    currentStatus,
				UpdatedAt: timestamppb.New(time.Now().UTC()),
			}
			if err := stream.Send(update); err != nil {
				return err
			}
			lastStatus = currentStatus
		}

		if currentStatus == "Paid" || currentStatus == "Failed" || currentStatus == "Cancelled" {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}
