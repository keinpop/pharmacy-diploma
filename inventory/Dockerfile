FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.work go.work.sum ./
COPY auth/go.mod auth/go.sum auth/
COPY inventory/go.mod inventory/go.sum inventory/
COPY sales/go.mod sales/go.sum sales/  

RUN go work sync

WORKDIR /app/inventory
COPY inventory/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /inventory .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /inventory /inventory
EXPOSE 50053
ENTRYPOINT ["/inventory"]