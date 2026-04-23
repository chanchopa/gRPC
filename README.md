# AP2 Assignment 2 — gRPC Migration (Order & Payment)

## Proto Repository
https://github.com/ArlanAidarov/ap2-protos

## Generated Code Repository
https://github.com/ArlanAidarov/ap2-generated
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

