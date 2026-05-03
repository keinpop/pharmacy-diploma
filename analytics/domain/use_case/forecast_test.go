package usecase_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pharmacy/analytics/domain"
	usecase "pharmacy/analytics/domain/use_case"
)

// newForecastReport is a helper to create a forecast Report with the given lookback.
func newForecastReport(id string, lookbackMonths int) *domain.Report {
	return &domain.Report{
		ID:     id,
		Type:   domain.TypeForecast,
		Status: domain.StatusProcessing,
		Params: map[string]interface{}{
			"lookback_months": lookbackMonths,
		},
	}
}

// buildMonthRows builds a slice of MonthlySalesRow for a single product, one per month,
// using the provided quantities in chronological order. baseMonth is the first month.
func buildMonthRows(productID, productName, group string, baseMonth time.Time, quantities []int) []usecase.MonthlySalesRow {
	rows := make([]usecase.MonthlySalesRow, len(quantities))
	for i, q := range quantities {
		rows[i] = usecase.MonthlySalesRow{
			ProductID:        productID,
			ProductName:      productName,
			TherapeuticGroup: group,
			Month:            baseMonth.AddDate(0, i, 0),
			Quantity:         q,
		}
	}
	return rows
}

// — tests: ProcessReport (forecast, 1-month lookback) —

func TestProcessReport_Forecast_1MonthLookback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		monthlySales     []usecase.MonthlySalesRow
		distinctProducts []usecase.ProductInfo
		expiringBatches  []usecase.ExpiringBatch
		stockByProduct   map[string][2]int // totalQty, reserved
		wantForecast     int               // expected forecast demand for "p1"
		seasonalFn       func(group string, month int) float64
	}{
		{
			name: "last month sales used as simple forecast, neutral seasonal",
			monthlySales: buildMonthRows("p1", "Aspirin", "painkiller",
				time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				[]int{80},
			),
			distinctProducts: []usecase.ProductInfo{
				{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
			},
			expiringBatches: []usecase.ExpiringBatch{},
			stockByProduct:  map[string][2]int{"p1": {200, 0}},
			seasonalFn:      func(_ string, _ int) float64 { return 1.0 },
			wantForecast:    80,
		},
		{
			name: "seasonal coefficient applied to last month",
			monthlySales: buildMonthRows("p1", "Aspirin", "painkiller",
				time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				[]int{100},
			),
			distinctProducts: []usecase.ProductInfo{
				{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
			},
			expiringBatches: []usecase.ExpiringBatch{},
			stockByProduct:  map[string][2]int{"p1": {500, 0}},
			// coefficient = 1.5 → forecast = 100 * 1.5 = 150
			seasonalFn:   func(_ string, _ int) float64 { return 1.5 },
			wantForecast: 150,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)

			report := newForecastReport("fc-1month", 1)

			events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
				Return(tc.monthlySales, nil)
			events.On("GetDistinctProducts", mock.Anything).
				Return(tc.distinctProducts, nil)
			inv.On("ListExpiringBatches", mock.Anything, 30).
				Return(tc.expiringBatches, nil)
			for productID, stock := range tc.stockByProduct {
				inv.On("GetStock", mock.Anything, productID).
					Return(stock[0], stock[1], nil)
			}
			repo.On("SaveResult", mock.Anything, "fc-1month", mock.MatchedBy(func(r interface{}) bool {
				result, ok := r.(*domain.ForecastResult)
				if !ok || len(result.Items) == 0 {
					return false
				}
				// Find item for "p1"
				for _, item := range result.Items {
					if item.ProductID == "p1" {
						return item.ForecastDemand == tc.wantForecast
					}
				}
				return false
			})).Return(nil)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, tc.seasonalFn)
			err := uc.ProcessReport(context.Background(), report)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (forecast, 6-month lookback) —

func TestProcessReport_Forecast_6MonthLookback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		quantities        []int
		stockQty          int
		reserved          int
		expiringQty       int
		seasonalFn        func(group string, month int) float64
		wantForecastRange [2]int // [min, max] inclusive
		wantConfidence    string
	}{
		{
			name: "double exponential smoothing with increasing trend",
			// Steady increase from 50 to 100
			quantities:        []int{50, 60, 70, 80, 90, 100},
			stockQty:          500,
			reserved:          0,
			expiringQty:       0,
			seasonalFn:        func(_ string, _ int) float64 { return 1.0 },
			wantForecastRange: [2]int{90, 130}, // trend-adjusted, ~level+trend * 1.0
			wantConfidence:    "medium",
		},
		{
			name:              "double exponential smoothing with stable series",
			quantities:        []int{100, 100, 100, 100, 100, 100},
			stockQty:          500,
			reserved:          0,
			expiringQty:       0,
			seasonalFn:        func(_ string, _ int) float64 { return 1.0 },
			wantForecastRange: [2]int{95, 110}, // should converge near 100
			wantConfidence:    "medium",
		},
		{
			name:        "seasonal factor doubles forecast",
			quantities:  []int{50, 50, 50, 50, 50, 50},
			stockQty:    0,
			reserved:    0,
			expiringQty: 0,
			// double seasonal coefficient
			seasonalFn:        func(_ string, _ int) float64 { return 2.0 },
			wantForecastRange: [2]int{90, 120}, // ~50 * 2.0 + trend adjustment
			wantConfidence:    "medium",
		},
	}

	baseMonth := time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)

			report := newForecastReport("fc-6month", 6)

			rows := buildMonthRows("p1", "Aspirin", "painkiller", baseMonth, tc.quantities)
			events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
				Return(rows, nil)
			events.On("GetDistinctProducts", mock.Anything).
				Return([]usecase.ProductInfo{
					{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
				}, nil)
			inv.On("ListExpiringBatches", mock.Anything, 30).
				Return([]usecase.ExpiringBatch{
					{ProductID: "p1", Quantity: tc.expiringQty},
				}, nil)
			inv.On("GetStock", mock.Anything, "p1").
				Return(tc.stockQty, tc.reserved, nil)
			repo.On("SaveResult", mock.Anything, "fc-6month", mock.MatchedBy(func(r interface{}) bool {
				result, ok := r.(*domain.ForecastResult)
				if !ok || len(result.Items) == 0 {
					return false
				}
				for _, item := range result.Items {
					if item.ProductID == "p1" {
						return item.ForecastDemand >= tc.wantForecastRange[0] &&
							item.ForecastDemand <= tc.wantForecastRange[1] &&
							item.Confidence == tc.wantConfidence
					}
				}
				return false
			})).Return(nil)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, tc.seasonalFn)
			err := uc.ProcessReport(context.Background(), report)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (forecast, 12-month lookback) —

func TestProcessReport_Forecast_12MonthLookback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		quantities        []int
		stockQty          int
		reserved          int
		expiringQty       int
		wantForecastRange [2]int
		wantConfidence    string
	}{
		{
			name: "triple exponential smoothing high confidence",
			// Simulate seasonal pattern: high in winter, low in summer
			quantities: []int{
				120, 110, 100, // Jan-Mar (winter/spring)
				80, 70, 60, // Apr-Jun (spring/summer)
				50, 60, 70, // Jul-Sep (summer/autumn)
				90, 100, 110, // Oct-Dec (autumn/winter)
			},
			stockQty:          300,
			reserved:          20,
			expiringQty:       0,
			wantForecastRange: [2]int{50, 150},
			wantConfidence:    "high",
		},
		{
			name:              "stable 12-month series produces stable forecast",
			quantities:        []int{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100},
			stockQty:          500,
			reserved:          0,
			expiringQty:       0,
			wantForecastRange: [2]int{80, 120}, // should stay near 100
			wantConfidence:    "high",
		},
		{
			name:              "recommended order accounts for expiring stock",
			quantities:        []int{50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50},
			stockQty:          100,
			reserved:          0,
			expiringQty:       80, // 80 units expiring; usable = 100-80 = 20
			wantForecastRange: [2]int{30, 70},
			wantConfidence:    "high",
		},
	}

	baseMonth := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)

			report := newForecastReport("fc-12month", 12)

			rows := buildMonthRows("p1", "Aspirin", "painkiller", baseMonth, tc.quantities)
			events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
				Return(rows, nil)
			events.On("GetDistinctProducts", mock.Anything).
				Return([]usecase.ProductInfo{
					{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
				}, nil)
			inv.On("ListExpiringBatches", mock.Anything, 30).
				Return([]usecase.ExpiringBatch{
					{ProductID: "p1", Quantity: tc.expiringQty},
				}, nil)
			inv.On("GetStock", mock.Anything, "p1").
				Return(tc.stockQty, tc.reserved, nil)

			repo.On("SaveResult", mock.Anything, "fc-12month", mock.MatchedBy(func(r interface{}) bool {
				result, ok := r.(*domain.ForecastResult)
				if !ok || len(result.Items) == 0 {
					return false
				}
				for _, item := range result.Items {
					if item.ProductID == "p1" {
						inRange := item.ForecastDemand >= tc.wantForecastRange[0] &&
							item.ForecastDemand <= tc.wantForecastRange[1]
						return inRange && item.Confidence == tc.wantConfidence
					}
				}
				return false
			})).Return(nil)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			err := uc.ProcessReport(context.Background(), report)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (forecast, empty sales data) —

func TestProcessReport_Forecast_EmptySalesData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		lookbackMonths int
		wantForecast   int
	}{
		{
			name:           "1-month lookback with no sales yields zero forecast",
			lookbackMonths: 1,
			wantForecast:   0,
		},
		{
			name:           "6-month lookback with no sales yields zero forecast",
			lookbackMonths: 6,
			wantForecast:   0,
		},
		{
			name:           "12-month lookback with no sales yields zero forecast",
			lookbackMonths: 12,
			wantForecast:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)

			report := newForecastReport("fc-empty", tc.lookbackMonths)

			// No monthly sales rows
			events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
				Return([]usecase.MonthlySalesRow{}, nil)
			// Product exists but has no sales history
			events.On("GetDistinctProducts", mock.Anything).
				Return([]usecase.ProductInfo{
					{ProductID: "p1", ProductName: "Orphan Drug", TherapeuticGroup: "unknown"},
				}, nil)
			inv.On("ListExpiringBatches", mock.Anything, 30).
				Return([]usecase.ExpiringBatch{}, nil)
			inv.On("GetStock", mock.Anything, "p1").
				Return(0, 0, nil)
			repo.On("SaveResult", mock.Anything, "fc-empty", mock.MatchedBy(func(r interface{}) bool {
				result, ok := r.(*domain.ForecastResult)
				if !ok || len(result.Items) == 0 {
					return false
				}
				for _, item := range result.Items {
					if item.ProductID == "p1" {
						return item.ForecastDemand == tc.wantForecast
					}
				}
				return false
			})).Return(nil)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			err := uc.ProcessReport(context.Background(), report)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (forecast, seasonal coefficient application) —

func TestProcessReport_Forecast_SeasonalCoefficients(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		lookbackMonths   int
		lastMonthQty     int
		seasonalCoeff    float64
		expectedForecast int
		description      string
	}{
		{
			name:             "high summer coefficient doubles cold_flu demand",
			lookbackMonths:   1,
			lastMonthQty:     100,
			seasonalCoeff:    0.4, // cold_flu summer = 0.4
			expectedForecast: 40,
			description:      "cold_flu in summer should be 100 * 0.4 = 40",
		},
		{
			name:             "winter spike for cold_flu",
			lookbackMonths:   1,
			lastMonthQty:     100,
			seasonalCoeff:    1.5, // cold_flu winter = 1.5
			expectedForecast: 150,
			description:      "cold_flu in winter should be 100 * 1.5 = 150",
		},
		{
			name:             "neutral coefficient for painkiller",
			lookbackMonths:   1,
			lastMonthQty:     200,
			seasonalCoeff:    1.0,
			expectedForecast: 200,
			description:      "painkiller always 1.0 seasonal coefficient",
		},
	}

	baseMonth := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)

			report := newForecastReport("fc-seasonal", tc.lookbackMonths)

			rows := buildMonthRows("p1", "Drug", "cold_flu", baseMonth, []int{tc.lastMonthQty})
			events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
				Return(rows, nil)
			events.On("GetDistinctProducts", mock.Anything).
				Return([]usecase.ProductInfo{
					{ProductID: "p1", ProductName: "Drug", TherapeuticGroup: "cold_flu"},
				}, nil)
			inv.On("ListExpiringBatches", mock.Anything, 30).
				Return([]usecase.ExpiringBatch{}, nil)
			inv.On("GetStock", mock.Anything, "p1").
				Return(0, 0, nil)

			capturedCoeff := tc.seasonalCoeff
			repo.On("SaveResult", mock.Anything, "fc-seasonal", mock.MatchedBy(func(r interface{}) bool {
				result, ok := r.(*domain.ForecastResult)
				if !ok || len(result.Items) == 0 {
					return false
				}
				for _, item := range result.Items {
					if item.ProductID == "p1" {
						expected := int(math.Round(float64(tc.lastMonthQty) * capturedCoeff))
						return item.ForecastDemand == expected
					}
				}
				return false
			})).Return(nil)

			coeff := tc.seasonalCoeff
			uc := usecase.NewAnalyticsUseCase(repo, events, inv, func(_ string, _ int) float64 {
				return coeff
			})
			err := uc.ProcessReport(context.Background(), report)

			require.NoError(t, err)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (forecast, inventory error resilience) —

func TestProcessReport_Forecast_InventoryErrorResilience(t *testing.T) {
	t.Parallel()
	// When inventory returns an error for GetStock, forecast should still succeed
	// (stock falls back to 0) per the implementation's skip logic.
	repo := new(MockReportRepo)
	events := new(MockEventRepo)
	inv := new(MockInventoryClient)

	baseMonth := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	report := newForecastReport("fc-inv-err", 1)

	rows := buildMonthRows("p1", "Aspirin", "painkiller", baseMonth, []int{50})
	events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
		Return(rows, nil)
	events.On("GetDistinctProducts", mock.Anything).
		Return([]usecase.ProductInfo{
			{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
		}, nil)
	inv.On("ListExpiringBatches", mock.Anything, 30).
		Return([]usecase.ExpiringBatch{}, nil)
	inv.On("GetStock", mock.Anything, "p1").
		Return(0, 0, errors.New("inventory service unavailable"))

	repo.On("SaveResult", mock.Anything, "fc-inv-err", mock.Anything).Return(nil)

	uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
	err := uc.ProcessReport(context.Background(), report)

	// Should not fail despite inventory error
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

// — tests: ProcessReport (forecast, expiring batches error) —

func TestProcessReport_Forecast_ExpiringBatchesError(t *testing.T) {
	t.Parallel()
	repo := new(MockReportRepo)
	events := new(MockEventRepo)
	inv := new(MockInventoryClient)

	baseMonth := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	report := newForecastReport("fc-exp-err", 1)

	rows := buildMonthRows("p1", "Aspirin", "painkiller", baseMonth, []int{50})
	events.On("GetMonthlySales", mock.Anything, mock.Anything, mock.Anything).
		Return(rows, nil)
	events.On("GetDistinctProducts", mock.Anything).
		Return([]usecase.ProductInfo{
			{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
		}, nil)
	inv.On("ListExpiringBatches", mock.Anything, 30).
		Return([]usecase.ExpiringBatch{}, errors.New("inventory unavailable"))

	uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
	err := uc.ProcessReport(context.Background(), report)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "list expiring batches")
}
