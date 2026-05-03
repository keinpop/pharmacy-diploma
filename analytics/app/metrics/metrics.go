// Package metrics — Prometheus-метрики analytics-сервиса.
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

const namespace = "analytics"

var (
	gRPCRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests handled by analytics service.",
	}, []string{"method", "code"})

	gRPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests handled by analytics service.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})

	// ReportsCreated — счётчик созданных отчётов по типу.
	ReportsCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "reports_created_total",
		Help:      "Total number of report jobs created, by type.",
	}, []string{"type"})

	// ReportsCompleted — счётчик завершённых отчётов с результатом.
	ReportsCompleted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "reports_completed_total",
		Help:      "Total number of completed reports, by status.",
	}, []string{"type", "status"})

	// KafkaEventsConsumed — счётчик обработанных Kafka-событий.
	KafkaEventsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "kafka_events_consumed_total",
		Help:      "Total number of Kafka events consumed by analytics service, by topic.",
	}, []string{"topic"})
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
