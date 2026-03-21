# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go workspace files
COPY go.work go.work.sum* ./

# Copy shared modules
COPY shared/ ./shared/

# Copy service code
COPY services/api-gateway/ ./services/api-gateway/

# Download dependencies
WORKDIR /app/services/api-gateway
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-gateway main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /api-gateway .

EXPOSE 8080

CMD ["./api-gateway"]
