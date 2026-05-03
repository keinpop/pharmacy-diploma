// Package metrics содержит Prometheus-метрики auth-сервиса и gRPC-интерсептор,
// собирающий счётчики/гистограммы по входящим вызовам.
package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const namespace = "auth"

var (
	// gRPCRequests считает все входящие RPC вызовы по методу и коду ответа.
	gRPCRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests handled by auth service.",
	}, []string{"method", "code"})

	// gRPCDuration — гистограмма длительности обработки RPC.
	gRPCDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests handled by auth service.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method"})

	// LoginAttempts — отдельная бизнес-метрика: попытки логина с результатом
	// success/failure. Полезна для дашбордов безопасности.
	LoginAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "login_attempts_total",
		Help:      "Total number of login attempts.",
	}, []string{"result"})

	// TokensIssued — счётчик выпущенных JWT (Register + Login).
	TokensIssued = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "tokens_issued_total",
		Help:      "Total number of JWT tokens issued.",
	})

	// TokensRevoked — счётчик отозванных токенов (Logout).
	TokensRevoked = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "tokens_revoked_total",
		Help:      "Total number of JWT tokens revoked via logout.",
	})
)

// UnaryServerInterceptor возвращает gRPC-интерсептор, инкрементирующий
// gRPCRequests/gRPCDuration на каждый вызов.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		gRPCRequests.WithLabelValues(info.FullMethod, code).Inc()
		gRPCDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

// Handler возвращает HTTP handler для /metrics (Prometheus scraper).
func Handler() http.Handler {
	return promhttp.Handler()
}

// IntToString — небольшой хелпер, чтобы не таскать strconv в вызывающие модули.
func IntToString(i int) string { return strconv.Itoa(i) }
