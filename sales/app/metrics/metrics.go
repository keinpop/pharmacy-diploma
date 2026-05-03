// Package metrics — Prometheus-метрики sales-сервиса.
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

const namespace = "sales"

var (
	gRPCRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests handled by sales service.",
	}, []string{"method", "code"})

	gRPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests handled by sales service.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})

	// SalesCreated — общее число успешно оформленных продаж.
	SalesCreated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "sales_created_total",
		Help:      "Total number of successfully created sales.",
	})

	// SaleAmount — гистограмма сумм продаж в денежных единицах.
	// По ней можно строить average/percentile дашборды.
	SaleAmount = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "sale_total_amount",
		Help:      "Distribution of sale total amounts.",
		Buckets:   []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000},
	})
)

// UnaryServerInterceptor собирает gRPC-метрики.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		gRPCRequests.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
		gRPCDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// Handler — http handler для /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
