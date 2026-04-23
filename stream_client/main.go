package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	pb "github.com/ArlanAidarov/ap2-generated/order"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	addr := flag.String("addr", "localhost:9090", "Order gRPC server address")
	orderID := flag.String("order", "", "Order ID to subscribe to")
	flag.Parse()

	if *orderID == "" {
		fmt.Fprintln(os.Stderr, "Usage: stream_client -order=<order-id> [-addr=localhost:9090]")
		os.Exit(1)
	}

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewOrderServiceClient(conn)

	stream, err := client.SubscribeToOrderUpdates(context.Background(), &pb.OrderRequest{
		OrderId: *orderID,
	})
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	fmt.Printf("Subscribed to order %s — waiting for status updates...\n", *orderID)

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("Stream closed by server.")
			return
		}
		if err != nil {
			log.Fatalf("recv: %v", err)
		}
		fmt.Printf("[UPDATE] order_id=%s  status=%s  at=%s\n",
			update.GetOrderId(),
			update.GetStatus(),
			update.GetUpdatedAt().AsTime().Format("15:04:05"),
		)
	}
}
