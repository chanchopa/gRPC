FROM golang:1.22-alpine AS builder

WORKDIR /workspace/notification-service
COPY notification-service/go.mod notification-service/go.sum ./
RUN go mod download

COPY notification-service/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/notification-service ./cmd/GoService

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
USER app
WORKDIR /home/app
COPY --from=builder /out/notification-service /usr/local/bin/notification-service
EXPOSE 8082
ENTRYPOINT ["/usr/local/bin/notification-service"]
