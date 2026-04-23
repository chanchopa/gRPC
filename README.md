# AP2 Assignment 2 — gRPC Migration (Order & Payment)

## Proto Repository
https://github.com/ArlanAidarov/ap2-protos

## Generated Code Repository
https://github.com/ArlanAidarov/ap2-generated

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        CLIENT                           │
│            HTTP REST  (curl / Postman)                  │
└──────────────────────┬──────────────────────────────────┘
                       │ POST /orders
                       ▼
┌──────────────────────────────────────────────────────────┐
│                   ORDER SERVICE                          │
│  Port 8080 — Gin REST (external API, unchanged)          │
│  Port 9090 — gRPC Server (SubscribeToOrderUpdates)       │
│                                                          │
│  domain/      ← UNCHANGED from Assignment 1              │
│  usecase/     ← UNCHANGED from Assignment 1              │
│  repository/  ← UNCHANGED from Assignment 1              │
│  transport/http/   ← UNCHANGED from Assignment 1         │
│  transport/grpc/   ← NEW: PaymentGRPCClient              │
│                         OrderGRPCServer (streaming)      │
└──────────────────────┬───────────────────────────────────┘
                       │ gRPC ProcessPayment
                       │ (replaces old HTTP call)
                       ▼
┌──────────────────────────────────────────────────────────┐
│                  PAYMENT SERVICE                         │
│  Port 8081 — Gin REST (kept for direct testing)          │
│  Port 9091 — gRPC Server (ProcessPayment)                │
│                                                          │
│  domain/      ← UNCHANGED from Assignment 1              │
│  usecase/     ← UNCHANGED from Assignment 1              │
│  repository/  ← UNCHANGED from Assignment 1              │
│  transport/http/   ← UNCHANGED from Assignment 1         │
│  transport/grpc/   ← NEW: PaymentGRPCServer              │
│                         LoggingInterceptor (bonus)       │
└──────────────────────────────────────────────────────────┘

Stream client (separate tool):
  stream_client → gRPC SubscribeToOrderUpdates → Order Service :9090
```

## Contract-First Flow

```
proto-repo  ──(GitHub Actions)──►  generated-repo
  payment/payment.proto              payment/payment.pb.go
  order/order.proto                  payment/payment_grpc.pb.go
                                     order/order.pb.go
                                     order/order_grpc.pb.go

Both services import via:
  require github.com/ArlanAidarov/ap2-generated v1.0.0
```

During local development the `replace` directive in each `go.mod` points to `../generated-repo` so you never need to push to GitHub just to compile.

---

## Prerequisites

- Go 1.22+
- PostgreSQL running locally
- Two databases created (run once):

```sql
-- in psql or pgAdmin
CREATE USER order_user   WITH PASSWORD 'order_pass';
CREATE USER payment_user WITH PASSWORD 'payment_pass';
CREATE DATABASE order_db   OWNER order_user;
CREATE DATABASE payment_db OWNER payment_user;
```

- Run migrations:

```bash
psql -U order_user   -d order_db   -f order-service/migrations/001_create_orders.sql
psql -U payment_user -d payment_db -f payment-service/migrations/001_create_payments.sql
```

---

## How to Run

### Step 1 — Download dependencies

```bash
cd generated-repo  && go mod tidy
cd ../order-service   && go mod tidy
cd ../payment-service && go mod tidy
cd ../stream_client   && go mod tidy
```

### Step 2 — Start Payment Service (terminal 1)

```bash
cd payment-service
# Windows PowerShell:
$env:PAYMENT_DB_DSN="postgres://payment_user:payment_pass@localhost:5432/payment_db?sslmode=disable"
$env:PAYMENT_HTTP_PORT="8081"
$env:PAYMENT_GRPC_PORT="9091"
go run ./cmd/GoService/main.go
```

### Step 3 — Start Order Service (terminal 2)

```bash
cd order-service
# Windows PowerShell:
$env:ORDER_DB_DSN="postgres://order_user:order_pass@localhost:5432/order_db?sslmode=disable"
$env:ORDER_HTTP_PORT="8080"
$env:ORDER_GRPC_PORT="9090"
$env:PAYMENT_GRPC_ADDR="localhost:9091"
go run ./cmd/GoService/main.go
```

---

## API Examples

### Create an order (normal, will be Paid)

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-1","item_name":"Laptop","amount":50000}'
```

### Create an order that will be Declined (amount > 100000)

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"cust-1","item_name":"Yacht","amount":200000}'
```

### Get an order

```bash
curl http://localhost:8080/orders/<id>
```

### Cancel a Pending order

```bash
curl -X PATCH http://localhost:8080/orders/<id>/cancel
```

### Idempotent order creation (bonus)

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-unique-key-001" \
  -d '{"customer_id":"cust-1","item_name":"Phone","amount":30000}'
# Sending the same request again returns the same order, no duplicate created
```

---

## Streaming Demo

In a **third terminal**, subscribe to an order's status updates in real time:

```bash
cd stream_client
go run main.go -order=<paste-order-id-here>
```

Then in another terminal create a new order. You will see the stream print each status change as it is committed to the database:

```
Subscribed to order abc-123 — waiting for status updates...
[UPDATE] order_id=abc-123  status=Pending   at=12:00:01
[UPDATE] order_id=abc-123  status=Paid      at=12:00:01
Stream closed by server.
```

---

## Bonus — gRPC Interceptor

The Payment Service registers a `LoggingInterceptor` on its gRPC server. Every incoming RPC call prints:

```
[gRPC] method=/payment.PaymentService/ProcessPayment duration=1.234ms
```

This is visible in the payment-service terminal.

---

## Grading Checklist

| Criterion | Implementation |
|---|---|
| Contract-First (proto repo + generated repo) | `proto-repo/` + `generated-repo/` + GitHub Actions workflow |
| gRPC server (Payment) | `payment-service/internal/transport/grpc/payment_server.go` |
| gRPC client (Order) | `order-service/internal/transport/grpc/payment_client.go` |
| Clean Architecture preserved | domain/ usecase/ repository/ untouched |
| Env vars, no hardcoded addresses | `.env` files + `os.Getenv` in `main.go` |
| Server-side streaming tied to DB | `order_stream_server.go` polls real DB rows |
| gRPC status codes | `codes.InvalidArgument`, `codes.Unavailable`, `codes.NotFound` |
| Logging interceptor (bonus +10%) | `payment-service/internal/transport/grpc/interceptor.go` |
