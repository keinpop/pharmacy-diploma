package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReportStatus string

const (
	StatusPending    ReportStatus = "pending"
	StatusProcessing ReportStatus = "processing"
	StatusReady      ReportStatus = "ready"
	StatusFailed     ReportStatus = "failed"
)

type ReportType string

const (
	TypeSalesReport    ReportType = "sales_report"
	TypeWriteOffReport ReportType = "write_off_report"
	TypeForecast       ReportType = "forecast"
	TypeWasteAnalysis  ReportType = "waste_analysis"
)

// Report represents a background analytics report.
type Report struct {
	ID          string
	Type        ReportType
	Status      ReportStatus
	Params      map[string]interface{}
	Result      interface{} // JSON-serializable
	CreatedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

// NewReport creates a new Report with a generated ID and pending status.
func NewReport(reportType ReportType, params map[string]interface{}) *Report {
	return &Report{
		ID:        uuid.New().String(),
		Type:      reportType,
		Status:    StatusPending,
		Params:    params,
		CreatedAt: time.Now().UTC(),
	}
}

// SalesReportResult holds aggregated sales data.
type SalesReportResult struct {
	Items         []SalesReportItem
	TotalRevenue  float64
	TotalQuantity int
}

type SalesReportItem struct {
	ProductID   string
	ProductName string
	QtySold     int
	Revenue     float64
	AvgPrice    float64
}

// WriteOffReportResult holds aggregated write-off data.
type WriteOffReportResult struct {
	Items           []WriteOffReportItem
	TotalLoss       float64
	TotalWrittenOff int
}

type WriteOffReportItem struct {
	ProductID     string
	ProductName   string
	QtyWrittenOff int
	LossAmount    float64
}

// ForecastResult holds demand forecast data.
type ForecastResult struct {
	Items          []ForecastItem
	Period         string // e.g. "2026-05"
	LookbackMonths int
}

type ForecastItem struct {
	ProductID         string
	ProductName       string
	TherapeuticGroup  string
	ForecastDemand    int
	CurrentStock      int
	ExpiringNextMonth int
	UsableStock       int
	WasteRatio        float64
	RecommendedOrder  int
	Confidence        string
	Note              string
}

// WasteAnalysisResult holds waste analysis data.
type WasteAnalysisResult struct {
	Items []WasteAnalysisItem
}

type WasteAnalysisItem struct {
	ProductID        string
	ProductName      string
	TherapeuticGroup string
	Received         int
	WrittenOff       int
	WasteRatio       float64
	LossAmount       float64
	Recommendation   string
}
