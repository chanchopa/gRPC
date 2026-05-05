# AP2 Assignment 3 - Event-Driven Architecture

**Author:** Taubakabyl Nurlybek (SE-2412)
**Course:** Advanced Programming 2
**Scope:** Lecture 5 (Message Queues) + Lecture 6 (Transactions & Reliability)

This project extends the gRPC-based Order/Payment microservices from Assignment 2 with an asynchronous **Notification Service** powered by **RabbitMQ**. The Payment Service publishes a `payment.completed` event after a payment is committed, and the Notification Service consumes it to "send" an email (it logs to stdout). The flow is fully decoupled, durable, idempotent, and ready for graceful shutdown.

---

## 1. Architecture

```
                   HTTP (Postman)
                         |
                         v
                 +---------------+
                 | Order Service |
                 |  (gRPC + HTTP)|
                 +-------+-------+
                         |  gRPC  ProcessPayment(order_id, amount)
                         |        + metadata: x-customer-email
                         v
                 +-----------------+         +-----------------+
                 | Payment Service |  --->   |    PostgreSQL   |
                 |  (gRPC + HTTP)  |  saves  | order_db        |
                 +--------+--------+ payment | payment_db      |
                          |                  | notification_db |
       publishes JSON     |                  +-----------------+
       (durable, persistent,
        publisher confirms)
                          v
                 +--------+----------+
                 |     RabbitMQ      |
                 | exchange: payments.exchange (direct)
                 | queue:    payment.completed (durable)
                 |     |                    |
                 |     | (3 attempts fail)  |
                 |     v                    |
                 | DLX: payments.dlx -> DLQ: payment.completed.dlq
                 +--------+----------+
                          |
              manual ACK  |   prefetch=10, autoAck=false
                          v
                 +-----------------------+
                 | Notification Service  |
                 |  (consumer worker)    |
                 |  idempotency: postgres|
                 +-----------------------+
                          |
                          v
                  Logs to stdout:
   [Notification] Sent email to user@example.com for Order #123. Amount: $99.99
```

The diagram is also available as `docs/architecture.md` (Mermaid).

---

## 2. Project Layout

```
AP2_Assignment3_Taubakabyl_Nurlybek_SE-2412/
├── docker-compose.yml          # Orchestrates 3 services + Postgres + RabbitMQ
├── db-init/                    # Postgres bootstrap (3 DBs + 3 users + tables)
├── docs/
│   └── architecture.md         # Mermaid diagram
├── postman/
│   └── AP2_Assignment3.postman_collection.json
├── generated-repo/             # gRPC stubs from Assignment 2 (kept)
├── order-service/              # HTTP+gRPC, calls payment via gRPC
│   ├── cmd/GoService/main.go
│   ├── internal/
│   │   ├── app/                # wiring + graceful shutdown
│   │   ├── domain/             # Order entity, ports
│   │   ├── repository/         # Postgres impl
│   │   ├── transport/grpc/     # gRPC streaming server, payment client
│   │   ├── transport/http/     # Gin handlers
│   │   └── usecase/
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── payment-service/            # HTTP+gRPC, RabbitMQ producer
│   ├── cmd/GoService/main.go
│   ├── internal/
│   │   ├── app/
│   │   ├── broker/             # RabbitMQ publisher (infra layer)
│   │   ├── domain/             # PaymentCompletedEvent + EventPublisher port
│   │   ├── repository/
│   │   ├── transport/grpc/
│   │   ├── transport/http/
│   │   └── usecase/
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
├── notification-service/       # NEW - RabbitMQ consumer
│   ├── cmd/GoService/main.go
│   ├── internal/
│   │   ├── app/
│   │   ├── broker/             # RabbitMQ consumer (manual ACK + DLQ)
│   │   ├── domain/             # PaymentCompletedEvent + ports
│   │   ├── repository/         # Postgres idempotency store
│   │   └── usecase/            # ProcessEvent + EmailNotifier
│   ├── migrations/
│   ├── Dockerfile
│   └── go.mod
└── stream_client/              # CLI to subscribe to order updates (from A2)
```

---

## 3. Event Contract

`payment-service` publishes JSON to exchange `payments.exchange` with routing key `payment.completed`:

```json
{
  "message_id":     "9f6c3f1e-2c3a-4d2c-8c11-7a36c2c0d5e2",
  "order_id":       "5c0c2bb0-...",
  "amount":         9999,
  "customer_email": "user@example.com",
  "status":         "Authorized"
}
```

* `amount` is in **cents** (int64) - the consumer formats it as `$99.99`.
* `message_id` is a fresh UUID for **every publish**, used by the consumer for idempotency.

---

## 4. Reliability and ACK Logic

### 4.1 Producer side (Payment Service)

* The exchange (`payments.exchange`, type `direct`) and the queue (`payment.completed`) are declared **durable**.
* Messages are published with `DeliveryMode = amqp.Persistent` so RabbitMQ writes them to disk.
* **Publisher Confirms** are enabled (`channel.Confirm(false)`) - after every publish the producer waits for the broker's `ack`. If the broker does not confirm within 5s, an error is returned to the caller.
* Publishing is wrapped behind the `domain.EventPublisher` interface so the use-case layer is independent of RabbitMQ. Swapping in NATS or Kafka would only touch `internal/broker/`.

### 4.2 Consumer side (Notification Service)

* `ch.Consume(queue, ..., autoAck=false, ...)` - **Manual ACKs are mandatory**.
* `ch.Qos(prefetch=10)` so a single slow message cannot block the queue while still bounding memory.
* Workflow per delivery:
  1. JSON-decode the body (malformed -> `Nack(requeue=false)` -> DLQ).
  2. Look up `message_id` in `processed_messages` (Postgres). If present, log "duplicate ignored" and `Ack`.
  3. Print the email log line.
  4. INSERT into `processed_messages` (PRIMARY KEY = `message_id`). If a unique-violation occurs from a concurrent worker, treat as success.
  5. `Ack(false)`.
* On transient errors the message is `Nack(requeue=true)`, retried in-process up to **MAX_RETRIES (default 3)**. After that it is `Nack(requeue=false)` and the broker routes it to the **Dead Letter Queue** via `x-dead-letter-exchange`.
* On permanent errors (`ErrPermanentFailure`, e.g. missing required fields) the message goes straight to the DLQ.

### 4.3 At-least-once delivery

If the consumer crashes between step 3 and step 5, the broker re-delivers the message because no `Ack` was sent. The `processed_messages` table makes the second processing a no-op, so the **end-to-end semantics are exactly-once observable behaviour on top of at-least-once delivery**.

---

## 5. Idempotency Strategy

| Layer                        | Mechanism                                                   |
|------------------------------|-------------------------------------------------------------|
| Producer (Payment Service)   | Generates a fresh `message_id` (UUID) per event             |
| Broker (RabbitMQ)            | `MessageId` header set + persistent delivery                |
| Consumer (Notification)      | `processed_messages` table - PK on `message_id`             |

Why a Postgres table instead of an in-memory map?

* Survives container restarts.
* Survives horizontal scale-out (multiple notification-service replicas).
* The unique-constraint violation is the source of truth - no race conditions.

Demo (after sending the same payment twice via Postman + a fixed `message_id` test):
```
[Notification] Sent email to user@example.com for Order #...  Amount: $9.99
[notification-service] duplicate event ignored message_id=...
```

---

## 6. Graceful Shutdown

Each service registers `signal.NotifyContext(SIGINT, SIGTERM)` in `main.go`. On signal:

1. The HTTP server's `Shutdown(ctx)` waits up to 15s for in-flight requests.
2. `grpcServer.GracefulStop()` (where applicable) drains active RPCs.
3. The RabbitMQ channel + connection are closed.
4. The Postgres connection pool is closed.

Test it: `docker compose stop notification-service` - the logs show every shutdown step in order, no goroutine leaks.

---

## 7. Bonus: Dead Letter Queue (DLQ)

The DLQ is wired automatically:

```
payments.exchange (direct) --routing key: payment.completed--> payment.completed
                                                                |
                                                  3 manual NACKs|
                                                                v
                                                    payments.dlx (direct)
                                                                |
                                                                v
                                                  payment.completed.dlq
```

To force a message into the DLQ (poison message demo): publish a message with an empty `order_id` or `message_id`. The consumer raises `ErrPermanentFailure` and immediately routes it to `payment.completed.dlq`.

You can inspect the DLQ in the RabbitMQ web UI:

* http://localhost:15672  (login: `guest` / `guest`)
* Queues -> `payment.completed.dlq` -> Get messages

---

## 8. How to Run (no extra software beyond GoLand + Postman + PostgreSQL)

There are **two** supported ways:

### 8.A One command via Docker Compose (recommended)

```
docker compose up --build
```

This starts:

| Component             | URL / Port                                    |
|-----------------------|-----------------------------------------------|
| Postgres              | `localhost:5432` (user `postgres` / `postgres`) |
| RabbitMQ broker       | `localhost:5672`                              |
| RabbitMQ Web UI       | http://localhost:15672  (guest/guest)         |
| Order Service HTTP    | http://localhost:8080                         |
| Payment Service HTTP  | http://localhost:8081                         |
| Notification HTTP     | http://localhost:8082/healthz                 |

The init scripts in `db-init/` create three databases (`order_db`, `payment_db`, `notification_db`) plus their owners on first start. Logs from `notification-service` show every `[Notification] Sent email ...` line.

To stop:
```
docker compose down            # keeps the volume
docker compose down -v         # wipes Postgres data too
```

### 8.B Run the Go services from GoLand against your own Postgres + RabbitMQ

1. **Start Postgres** (your local install).
   Run as a superuser:
   ```sql
   CREATE USER order_user        WITH PASSWORD '1234';
   CREATE USER payment_user      WITH PASSWORD '1234';
   CREATE USER notification_user WITH PASSWORD '1234';
   CREATE DATABASE order_db        OWNER order_user;
   CREATE DATABASE payment_db      OWNER payment_user;
   CREATE DATABASE notification_db OWNER notification_user;
   ```
   Then run the three migration files (`order-service/migrations/001_*.sql`, etc.) against each database.

2. **Start RabbitMQ** *only* via Docker (the assignment forbids forcing the grader to install it):
   ```
   docker compose up rabbitmq
   ```
   (The grader can also run `docker run --rm -p 5672:5672 -p 15672:15672 rabbitmq:3.13-management-alpine`.)

3. **Open the project in GoLand.** Each service has its own `go.mod`:
   * `order-service`
   * `payment-service`
   * `notification-service`

   GoLand will detect all three modules. For each, open `cmd/GoService/main.go` and click **Run** (or create a Run Configuration). The `.env` files inside each service folder list the variables you can set under "Environment variables" - the defaults already point at `localhost`.

   Run order: `payment-service` -> `notification-service` -> `order-service` (so the gRPC client in `order-service` finds the payment server when it dials).

4. **Use Postman** to drive the system. Import `postman/AP2_Assignment3.postman_collection.json` for ready-made requests.

---

## 9. Testing the Flow with Postman

### 9.1 Create an order

`POST http://localhost:8080/orders`
Headers (optional): `Idempotency-Key: any-string-of-yours`
Body:
```json
{
  "customer_id":    "11111111-1111-1111-1111-111111111111",
  "customer_email": "user@example.com",
  "item_name":      "Notebook",
  "amount":         9999
}
```

Expected: `201 Created`. The `notification-service` log immediately prints:
```
[Notification] Sent email to user@example.com for Order #<uuid>. Amount: $99.99
```

### 9.2 Direct payment (skip the order service)

`POST http://localhost:8081/payments`
```json
{
  "order_id":       "demo-order-1",
  "amount":         4999,
  "customer_email": "alice@example.com"
}
```

### 9.3 Idempotency test

Sending the same order twice with the same `Idempotency-Key` returns the existing order; only one notification appears.

### 9.4 Reliability test

1. `docker compose stop notification-service`
2. Create 3 orders via Postman.
3. Open http://localhost:15672 -> Queues -> `payment.completed`. You see 3 ready messages.
4. `docker compose start notification-service`. Within seconds the queue drains and 3 `[Notification]` lines appear.

### 9.5 DLQ test (poison message)

`POST http://localhost:8081/payments` with `order_id` set to `""` is rejected by validation, so to demo the DLQ you can publish a malformed message directly from the RabbitMQ web UI:

* Exchanges -> `payments.exchange` -> Publish message
* Routing key: `payment.completed`
* Payload: `{"message_id":"","order_id":"","amount":0,"customer_email":"","status":""}`
* Properties: `delivery_mode = 2`

The consumer logs `permanent failure, routing to DLQ` and the message lands in `payment.completed.dlq`.

---

## 10. Mapping to the Grading Rubric

| Criterion          | Weight | Where it lives                                                                                              |
|--------------------|-------:|-------------------------------------------------------------------------------------------------------------|
| Messaging Logic    |   30 % | `payment-service/internal/broker/rabbitmq_publisher.go` + `notification-service/internal/broker/rabbitmq_consumer.go` |
| Reliability & ACKs |   20 % | manual `Ack`/`Nack`, durable queue, persistent delivery, publisher confirms                                 |
| Idempotency        |   20 % | `processed_messages` table + `notification-service/internal/repository/postgres_idempotency.go`             |
| Docker & Lifecycle |   20 % | `docker-compose.yml` (healthchecks, depends_on) + `signal.NotifyContext` + `Shutdown` in every service      |
| Documentation      |   10 % | this README + `docs/architecture.md`                                                                        |
| Bonus DLQ          |  +10 % | `x-dead-letter-exchange`/`x-dead-letter-routing-key` on the main queue + DLX/DLQ declaration                |

---

## 11. Engineering Decisions

* **RabbitMQ over NATS** - the assignment lists both; RabbitMQ is the more common choice in the recommended Go tutorial and gives us per-message persistence + a built-in DLX, which makes the DLQ bonus straightforward.
* **JSON over ProtoBuf** for the event payload - human-readable in the RabbitMQ UI and decouples the consumer from the gRPC stubs (the Notification Service does not import `generated-repo`).
* **Email passed via gRPC metadata header** instead of regenerating the proto - keeps the diff small and demonstrates that domain data can flow through gRPC metadata without touching the contract.
* **Idempotency in Postgres** instead of an in-memory map - survives restarts and supports horizontal scaling later.
* **Three separate databases** - one per service, matching the "database per service" pattern; the Notification Service only knows about `notification_db.processed_messages` and never reads order/payment tables.
