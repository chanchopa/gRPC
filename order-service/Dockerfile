FROM golang:1.22-alpine AS builder

WORKDIR /workspace

COPY generated-repo/ ./generated-repo/

WORKDIR /workspace/order-service
COPY order-service/go.mod order-service/go.sum ./
RUN go mod download

COPY order-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/order-service ./cmd/GoService

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
WORKDIR /home/app
COPY --from=builder /out/order-service /usr/local/bin/order-service
EXPOSE 8080 9090
ENTRYPOINT ["/usr/local/bin/order-service"]
