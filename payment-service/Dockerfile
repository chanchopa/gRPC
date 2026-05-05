FROM golang:1.22-alpine AS builder

WORKDIR /workspace

COPY generated-repo/ ./generated-repo/

WORKDIR /workspace/payment-service
COPY payment-service/go.mod payment-service/go.sum ./
RUN go mod download

COPY payment-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/payment-service ./cmd/GoService

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
WORKDIR /home/app
COPY --from=builder /out/payment-service /usr/local/bin/payment-service
EXPOSE 8081 9091
ENTRYPOINT ["/usr/local/bin/payment-service"]
