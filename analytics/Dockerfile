FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.work ./
COPY auth/go.mod auth/go.sum auth/
COPY inventory/go.mod inventory/go.sum inventory/
COPY sales/go.mod sales/go.sum sales/
COPY analytics/go.mod analytics/go.sum analytics/

RUN go work sync

WORKDIR /app/analytics
COPY analytics/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /analytics .

FROM alpine:3.19
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /analytics /analytics
CMD ["/analytics"]
