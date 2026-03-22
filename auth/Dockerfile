FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth_service .

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /auth_service .
EXPOSE 50051
CMD ["./auth_service"]
