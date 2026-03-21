# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go workspace files
COPY go.work go.work.sum* ./

# Copy shared modules
COPY shared/ ./shared/

# Copy service code
COPY services/auth-service/ ./services/auth-service/

# Download dependencies
WORKDIR /app/services/auth-service
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /auth-service .

EXPOSE 8081

CMD ["./auth-service"]
