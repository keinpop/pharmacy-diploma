// Package metrics — Prometheus-метрики inventory-сервиса.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const namespace = "inventory"

var (
	gRPCRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests handled by inventory service.",
	}, []string{"method", "code"})

	gRPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests handled by inventory service.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})

	// BatchesReceived — счётчик принятых партий.
	BatchesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "batches_received_total",
		Help:      "Total number of received batches.",
	})

	// BatchesWrittenOff — счётчик списанных партий.
	BatchesWrittenOff = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "batches_written_off_total",
		Help:      "Total number of written-off (expired) batches.",
	})

	// StockDeductions — счётчик операций списания со склада (продажи).
	StockDeductions = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "stock_deductions_total",
		Help:      "Total number of stock deduction operations.",
	})
)

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		gRPCRequests.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
		gRPCDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func Handler() http.Handler {
	return promhttp.Handler()
}
