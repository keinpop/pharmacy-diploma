package grpc

import (
	"context"

	"pharmacy/analytics/domain"
)

// AnalyticsUC is the interface the gRPC handler uses to invoke business logic.
type AnalyticsUC interface {
	CreateSalesReport(ctx context.Context, period string) (string, error)
	GetSalesReport(ctx context.Context, id string) (*domain.Report, error)
	CreateWriteOffReport(ctx context.Context, period string) (string, error)
	GetWriteOffReport(ctx context.Context, id string) (*domain.Report, error)
	CreateForecast(ctx context.Context, lookbackMonths int) (string, error)
	GetForecast(ctx context.Context, id string) (*domain.Report, error)
	CreateWasteAnalysis(ctx context.Context, period string) (string, error)
	GetWasteAnalysis(ctx context.Context, id string) (*domain.Report, error)
}
