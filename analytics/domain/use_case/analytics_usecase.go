package usecase

import (
	"context"
	"fmt"
	"time"

	"pharmacy/analytics/app/metrics"
	"pharmacy/analytics/domain"
)

// AnalyticsUseCase implements the core analytics business logic.
type AnalyticsUseCase struct {
	reports    ReportRepository
	events     EventRepository
	inventory  InventoryClient
	seasonalFn func(group string, month int) float64
}

// NewAnalyticsUseCase creates a new AnalyticsUseCase.
func NewAnalyticsUseCase(
	reports ReportRepository,
	events EventRepository,
	inventory InventoryClient,
	seasonalFn func(group string, month int) float64,
) *AnalyticsUseCase {
	return &AnalyticsUseCase{
		reports:    reports,
		events:     events,
		inventory:  inventory,
		seasonalFn: seasonalFn,
	}
}

// CreateSalesReport creates a pending sales report and returns its ID.
func (uc *AnalyticsUseCase) CreateSalesReport(ctx context.Context, period string) (string, error) {
	report := domain.NewReport(domain.TypeSalesReport, map[string]interface{}{
		"period": period,
	})
	if err := uc.reports.Create(ctx, report); err != nil {
		return "", fmt.Errorf("create sales report: %w", err)
	}
	metrics.ReportsCreated.WithLabelValues(string(domain.TypeSalesReport)).Inc()
	return report.ID, nil
}

// GetSalesReport retrieves a sales report by ID.
func (uc *AnalyticsUseCase) GetSalesReport(ctx context.Context, id string) (*domain.Report, error) {
	return uc.reports.GetByID(ctx, id)
}

// CreateWriteOffReport creates a pending write-off report and returns its ID.
func (uc *AnalyticsUseCase) CreateWriteOffReport(ctx context.Context, period string) (string, error) {
	report := domain.NewReport(domain.TypeWriteOffReport, map[string]interface{}{
		"period": period,
	})
	if err := uc.reports.Create(ctx, report); err != nil {
		return "", fmt.Errorf("create write-off report: %w", err)
	}
	metrics.ReportsCreated.WithLabelValues(string(domain.TypeWriteOffReport)).Inc()
	return report.ID, nil
}

// GetWriteOffReport retrieves a write-off report by ID.
func (uc *AnalyticsUseCase) GetWriteOffReport(ctx context.Context, id string) (*domain.Report, error) {
	return uc.reports.GetByID(ctx, id)
}

// CreateForecast creates a pending forecast report and returns its ID.
func (uc *AnalyticsUseCase) CreateForecast(ctx context.Context, lookbackMonths int) (string, error) {
	report := domain.NewReport(domain.TypeForecast, map[string]interface{}{
		"lookback_months": lookbackMonths,
	})
	if err := uc.reports.Create(ctx, report); err != nil {
		return "", fmt.Errorf("create forecast: %w", err)
	}
	metrics.ReportsCreated.WithLabelValues(string(domain.TypeForecast)).Inc()
	return report.ID, nil
}

// GetForecast retrieves a forecast report by ID.
func (uc *AnalyticsUseCase) GetForecast(ctx context.Context, id string) (*domain.Report, error) {
	return uc.reports.GetByID(ctx, id)
}

// CreateWasteAnalysis creates a pending waste analysis report and returns its ID.
func (uc *AnalyticsUseCase) CreateWasteAnalysis(ctx context.Context, period string) (string, error) {
	report := domain.NewReport(domain.TypeWasteAnalysis, map[string]interface{}{
		"period": period,
	})
	if err := uc.reports.Create(ctx, report); err != nil {
		return "", fmt.Errorf("create waste analysis: %w", err)
	}
	metrics.ReportsCreated.WithLabelValues(string(domain.TypeWasteAnalysis)).Inc()
	return report.ID, nil
}

// GetWasteAnalysis retrieves a waste analysis report by ID.
func (uc *AnalyticsUseCase) GetWasteAnalysis(ctx context.Context, id string) (*domain.Report, error) {
	return uc.reports.GetByID(ctx, id)
}

// ProcessReport dispatches processing based on report type.
// Called by the worker goroutine.
func (uc *AnalyticsUseCase) ProcessReport(ctx context.Context, report *domain.Report) error {
	switch report.Type {
	case domain.TypeSalesReport:
		return uc.processSalesReport(ctx, report)
	case domain.TypeWriteOffReport:
		return uc.processWriteOffReport(ctx, report)
	case domain.TypeForecast:
		return uc.processForecast(ctx, report)
	case domain.TypeWasteAnalysis:
		return uc.processWasteAnalysis(ctx, report)
	default:
		return fmt.Errorf("unknown report type: %s", report.Type)
	}
}

// processSalesReport computes aggregated sales data for the given period.
func (uc *AnalyticsUseCase) processSalesReport(ctx context.Context, report *domain.Report) error {
	period, _ := report.Params["period"].(string)
	from, to := periodToTimeRange(period)

	rows, err := uc.events.GetSalesReport(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get sales report rows: %w", err)
	}

	result := &domain.SalesReportResult{}
	for _, row := range rows {
		result.Items = append(result.Items, domain.SalesReportItem{
			ProductID:   row.ProductID,
			ProductName: row.ProductName,
			QtySold:     row.TotalQty,
			Revenue:     row.Revenue,
			AvgPrice:    row.AvgPrice,
		})
		result.TotalRevenue += row.Revenue
		result.TotalQuantity += row.TotalQty
	}

	return uc.reports.SaveResult(ctx, report.ID, result)
}

// processWriteOffReport computes aggregated write-off data for the given period.
func (uc *AnalyticsUseCase) processWriteOffReport(ctx context.Context, report *domain.Report) error {
	period, _ := report.Params["period"].(string)
	from, to := periodToTimeRange(period)

	rows, err := uc.events.GetWriteOffReport(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get write-off report rows: %w", err)
	}

	result := &domain.WriteOffReportResult{}
	for _, row := range rows {
		result.Items = append(result.Items, domain.WriteOffReportItem{
			ProductID:     row.ProductID,
			ProductName:   row.ProductName,
			QtyWrittenOff: row.TotalQty,
			// LossAmount computed from write_off_events retail_price — not available in this row type,
			// so we leave it zero unless extended; the spec does not require a price join here.
		})
		result.TotalWrittenOff += row.TotalQty
	}

	return uc.reports.SaveResult(ctx, report.ID, result)
}

// processWasteAnalysis computes waste ratios per product for the given period.
func (uc *AnalyticsUseCase) processWasteAnalysis(ctx context.Context, report *domain.Report) error {
	period, _ := report.Params["period"].(string)
	from, to := periodToTimeRange(period)

	receivedRows, err := uc.events.GetMonthlyReceived(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get monthly received: %w", err)
	}
	writeOffRows, err := uc.events.GetMonthlyWriteOffs(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get monthly write-offs: %w", err)
	}
	products, err := uc.events.GetDistinctProducts(ctx)
	if err != nil {
		return fmt.Errorf("get distinct products: %w", err)
	}

	// Aggregate per product
	type aggregate struct {
		received   int
		writtenOff int
	}
	agg := make(map[string]*aggregate)
	for _, r := range receivedRows {
		if _, ok := agg[r.ProductID]; !ok {
			agg[r.ProductID] = &aggregate{}
		}
		agg[r.ProductID].received += r.Quantity
	}
	for _, w := range writeOffRows {
		if _, ok := agg[w.ProductID]; !ok {
			agg[w.ProductID] = &aggregate{}
		}
		agg[w.ProductID].writtenOff += w.Quantity
	}

	// Build product info map
	productMap := make(map[string]ProductInfo)
	for _, p := range products {
		productMap[p.ProductID] = p
	}

	result := &domain.WasteAnalysisResult{}
	for productID, a := range agg {
		if a.received == 0 {
			continue
		}
		wasteRatio := float64(a.writtenOff) / float64(a.received)
		info := productMap[productID]
		recommendation := buildRecommendation(wasteRatio)
		result.Items = append(result.Items, domain.WasteAnalysisItem{
			ProductID:        productID,
			ProductName:      info.ProductName,
			TherapeuticGroup: info.TherapeuticGroup,
			Received:         a.received,
			WrittenOff:       a.writtenOff,
			WasteRatio:       wasteRatio,
			Recommendation:   recommendation,
		})
	}

	return uc.reports.SaveResult(ctx, report.ID, result)
}

// processForecast delegates to forecast.go.
func (uc *AnalyticsUseCase) processForecast(ctx context.Context, report *domain.Report) error {
	var lookback int
	switch v := report.Params["lookback_months"].(type) {
	case float64:
		lookback = int(v)
	case int:
		lookback = v
	default:
		lookback = 1
	}
	return runForecast(ctx, report.ID, lookback, uc.events, uc.inventory, uc.reports, uc.seasonalFn)
}

// periodToTimeRange converts a period string to from/to time range.
func periodToTimeRange(period string) (from, to time.Time) {
	to = time.Now().UTC()
	switch period {
	case "MONTH":
		from = to.AddDate(0, 0, -30)
	case "HALF_YEAR":
		from = to.AddDate(0, 0, -180)
	case "YEAR":
		from = to.AddDate(0, 0, -365)
	default:
		from = to.AddDate(0, 0, -30)
	}
	return
}

// buildRecommendation returns a textual recommendation based on waste ratio.
func buildRecommendation(wasteRatio float64) string {
	switch {
	case wasteRatio >= 0.3:
		return "High waste ratio — consider reducing order quantities or improving storage conditions"
	case wasteRatio >= 0.1:
		return "Moderate waste — review expiry dates and ordering frequency"
	default:
		return "Waste within acceptable range"
	}
}
