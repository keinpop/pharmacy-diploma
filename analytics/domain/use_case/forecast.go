package usecase

import (
	"context"
	"fmt"
	"math"
	"time"

	"pharmacy/analytics/domain"
)

// runForecast executes the demand forecast and saves the result.
func runForecast(
	ctx context.Context,
	reportID string,
	lookbackMonths int,
	events EventRepository,
	inventory InventoryClient,
	reports ReportRepository,
	seasonalFn func(group string, month int) float64,
) error {
	// Determine time range based on lookback
	to := time.Now().UTC()
	from := to.AddDate(0, -lookbackMonths, 0)

	monthlySales, err := events.GetMonthlySales(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get monthly sales: %w", err)
	}

	// Group by product
	type productData struct {
		name             string
		therapeuticGroup string
		// ordered monthly quantities
		months []float64
	}
	// Build ordered list of unique months (for ordering)
	monthSet := make(map[time.Time]bool)
	for _, r := range monthlySales {
		monthSet[r.Month] = true
	}
	orderedMonths := sortedMonths(monthSet)

	// Build per-product time series
	productMap := make(map[string]*productData)
	for _, r := range monthlySales {
		pd, ok := productMap[r.ProductID]
		if !ok {
			pd = &productData{
				name:             r.ProductName,
				therapeuticGroup: r.TherapeuticGroup,
				months:           make([]float64, len(orderedMonths)),
			}
			productMap[r.ProductID] = pd
		}
		for i, m := range orderedMonths {
			if m.Equal(r.Month) {
				pd.months[i] += float64(r.Quantity)
			}
		}
	}

	// Get all distinct products (including those with no sales)
	products, err := events.GetDistinctProducts(ctx)
	if err != nil {
		return fmt.Errorf("get distinct products: %w", err)
	}
	// Ensure all products appear in productMap
	for _, p := range products {
		if _, ok := productMap[p.ProductID]; !ok {
			productMap[p.ProductID] = &productData{
				name:             p.ProductName,
				therapeuticGroup: p.TherapeuticGroup,
				months:           make([]float64, len(orderedMonths)),
			}
		}
	}

	// Get expiring batches (next 30 days)
	expiringBatches, err := inventory.ListExpiringBatches(ctx, 30)
	if err != nil {
		return fmt.Errorf("list expiring batches: %w", err)
	}
	expiringMap := make(map[string]int)
	for _, b := range expiringBatches {
		expiringMap[b.ProductID] += b.Quantity
	}

	// Determine confidence and forecast month
	forecastMonth := to.AddDate(0, 1, 0)
	period := forecastMonth.Format("2006-01")
	confidence := confidenceLevel(lookbackMonths)

	result := &domain.ForecastResult{
		Period:         period,
		LookbackMonths: lookbackMonths,
	}

	for productID, pd := range productMap {
		// Compute forecast demand
		forecastDemand := computeForecastDemand(pd.months, lookbackMonths, pd.therapeuticGroup, forecastMonth.Month(), seasonalFn)

		// Get current stock
		totalQty, reserved, err := inventory.GetStock(ctx, productID)
		if err != nil {
			// Skip if inventory unavailable, don't fail the whole report
			totalQty = 0
			reserved = 0
		}
		currentStock := totalQty - reserved

		expiringNextMonth := expiringMap[productID]
		usableStock := currentStock - expiringNextMonth
		if usableStock < 0 {
			usableStock = 0
		}

		recommendedOrder := int(math.Max(0, float64(forecastDemand)-float64(usableStock)))

		var wasteRatio float64
		if totalQty > 0 {
			wasteRatio = float64(expiringNextMonth) / float64(totalQty)
		}

		note := ""
		if expiringNextMonth > 0 {
			note = fmt.Sprintf("%d units expiring in next 30 days", expiringNextMonth)
		}

		result.Items = append(result.Items, domain.ForecastItem{
			ProductID:         productID,
			ProductName:       pd.name,
			TherapeuticGroup:  pd.therapeuticGroup,
			ForecastDemand:    forecastDemand,
			CurrentStock:      totalQty,
			ExpiringNextMonth: expiringNextMonth,
			UsableStock:       usableStock,
			WasteRatio:        wasteRatio,
			RecommendedOrder:  recommendedOrder,
			Confidence:        confidence,
			Note:              note,
		})
	}

	return reports.SaveResult(ctx, reportID, result)
}

// computeForecastDemand computes the forecasted demand using the appropriate smoothing method.
func computeForecastDemand(
	months []float64,
	lookbackMonths int,
	therapeuticGroup string,
	forecastMonth time.Month,
	seasonalFn func(group string, month int) float64,
) int {
	if len(months) == 0 {
		return 0
	}

	var forecast float64

	switch {
	case lookbackMonths == 1:
		// Simple: use last month's quantity with seasonal adjustment
		forecast = months[len(months)-1]
		seasonal := seasonalFn(therapeuticGroup, int(forecastMonth))
		forecast *= seasonal

	case lookbackMonths <= 6:
		// Double exponential smoothing (Holt's method)
		alpha := 0.3
		beta := 0.1
		level, trend := holtDoubleSmoothing(months, alpha, beta)
		forecast = level + trend
		// Apply seasonal coefficient
		seasonal := seasonalFn(therapeuticGroup, int(forecastMonth))
		forecast *= seasonal

	default:
		// Triple exponential smoothing (Holt-Winters) — no seasonal coefficient applied externally
		alpha := 0.3
		beta := 0.1
		gamma := 0.3
		seasonLength := 12
		forecast = holtWintersTriple(months, alpha, beta, gamma, seasonLength)
	}

	if forecast < 0 {
		forecast = 0
	}
	return int(math.Round(forecast))
}

// holtDoubleSmoothing applies Holt's double exponential smoothing and returns (level, trend).
func holtDoubleSmoothing(data []float64, alpha, beta float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	if len(data) == 1 {
		return data[0], 0
	}

	level := data[0]
	trend := data[1] - data[0]

	for i := 1; i < len(data); i++ {
		prevLevel := level
		level = alpha*data[i] + (1-alpha)*(level+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
	}
	return level, trend
}

// holtWintersTriple applies Holt-Winters triple exponential smoothing and returns a one-step-ahead forecast.
func holtWintersTriple(data []float64, alpha, beta, gamma float64, seasonLength int) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}
	if n < seasonLength {
		// Not enough data for full seasonal cycle — fall back to double smoothing
		level, trend := holtDoubleSmoothing(data, alpha, beta)
		return level + trend
	}

	// Initialize seasonal indices from first season
	seasonal := make([]float64, seasonLength)
	var firstSeasonAvg float64
	for i := 0; i < seasonLength; i++ {
		firstSeasonAvg += data[i]
	}
	firstSeasonAvg /= float64(seasonLength)
	for i := 0; i < seasonLength; i++ {
		if firstSeasonAvg != 0 {
			seasonal[i] = data[i] / firstSeasonAvg
		} else {
			seasonal[i] = 1.0
		}
	}

	// Initialize level and trend
	level := firstSeasonAvg
	trend := 0.0
	if n >= 2*seasonLength {
		var secondSeasonAvg float64
		for i := seasonLength; i < 2*seasonLength; i++ {
			secondSeasonAvg += data[i]
		}
		secondSeasonAvg /= float64(seasonLength)
		trend = (secondSeasonAvg - firstSeasonAvg) / float64(seasonLength)
	}

	// Run Holt-Winters iterations
	for i := 0; i < n; i++ {
		s := seasonal[i%seasonLength]
		if s == 0 {
			s = 1.0
		}
		prevLevel := level
		level = alpha*(data[i]/s) + (1-alpha)*(level+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
		seasonal[i%seasonLength] = gamma*(data[i]/level) + (1-gamma)*s
	}

	// One-step-ahead forecast
	nextSeasonal := seasonal[n%seasonLength]
	return (level + trend) * nextSeasonal
}

// confidenceLevel returns a confidence string based on lookback months.
func confidenceLevel(lookbackMonths int) string {
	switch {
	case lookbackMonths >= 12:
		return "high"
	case lookbackMonths >= 6:
		return "medium"
	default:
		return "low"
	}
}

// sortedMonths returns a sorted slice of month timestamps from a set.
func sortedMonths(monthSet map[time.Time]bool) []time.Time {
	months := make([]time.Time, 0, len(monthSet))
	for m := range monthSet {
		months = append(months, m)
	}
	// Simple insertion sort (small data)
	for i := 1; i < len(months); i++ {
		key := months[i]
		j := i - 1
		for j >= 0 && months[j].After(key) {
			months[j+1] = months[j]
			j--
		}
		months[j+1] = key
	}
	return months
}
