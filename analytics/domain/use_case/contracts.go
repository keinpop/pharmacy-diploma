package usecase

import (
	"context"
	"time"

	"pharmacy/analytics/domain"
)

// ReportRepository persists analytics reports in PostgreSQL.
type ReportRepository interface {
	Create(ctx context.Context, report *domain.Report) error
	GetByID(ctx context.Context, id string) (*domain.Report, error)
	ListPending(ctx context.Context) ([]*domain.Report, error)
	UpdateStatus(ctx context.Context, id string, status domain.ReportStatus) error
	SaveResult(ctx context.Context, id string, result interface{}) error
	SaveError(ctx context.Context, id string, errMsg string) error
}

// EventRepository queries aggregated event data from ClickHouse.
type EventRepository interface {
	// Sales aggregations
	GetMonthlySales(ctx context.Context, from, to time.Time) ([]MonthlySalesRow, error)
	GetSalesReport(ctx context.Context, from, to time.Time) ([]SalesReportRow, error)
	// Write-off aggregations
	GetWriteOffReport(ctx context.Context, from, to time.Time) ([]WriteOffReportRow, error)
	GetMonthlyWriteOffs(ctx context.Context, from, to time.Time) ([]MonthlyWriteOffRow, error)
	// Received aggregations
	GetMonthlyReceived(ctx context.Context, from, to time.Time) ([]MonthlyReceivedRow, error)
	// Product info from events
	GetDistinctProducts(ctx context.Context) ([]ProductInfo, error)
}

// InventoryClient is the gRPC client interface for the Inventory service.
type InventoryClient interface {
	GetStock(ctx context.Context, productID string) (totalQty int, reserved int, err error)
	ListExpiringBatches(ctx context.Context, daysAhead int) ([]ExpiringBatch, error)
}

// Row types for ClickHouse queries.

type MonthlySalesRow struct {
	ProductID        string
	ProductName      string
	TherapeuticGroup string
	Month            time.Time // first day of month
	Quantity         int
	Revenue          float64
}

type SalesReportRow struct {
	ProductID   string
	ProductName string
	TotalQty    int
	Revenue     float64
	AvgPrice    float64
}

type WriteOffReportRow struct {
	ProductID   string
	ProductName string
	TotalQty    int
}

type MonthlyWriteOffRow struct {
	ProductID string
	Month     time.Time
	Quantity  int
}

type MonthlyReceivedRow struct {
	ProductID string
	Month     time.Time
	Quantity  int
}

type ProductInfo struct {
	ProductID        string
	ProductName      string
	TherapeuticGroup string
}

type ExpiringBatch struct {
	ProductID string
	Quantity  int
	ExpiresAt time.Time
}
