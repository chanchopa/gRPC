package grpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
)

func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)
	if err != nil {
		log.Printf("[gRPC] method=%s duration=%s error=%v", info.FullMethod, duration, err)
	} else {
		log.Printf("[gRPC] method=%s duration=%s", info.FullMethod, duration)
	}
	return resp, err
}
