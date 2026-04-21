package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pharmacy/analytics/domain"
	usecase "pharmacy/analytics/domain/use_case"
)

// — mock: ReportRepository —

type MockReportRepo struct{ mock.Mock }

func (m *MockReportRepo) Create(ctx context.Context, report *domain.Report) error {
	return m.Called(ctx, report).Error(0)
}
func (m *MockReportRepo) GetByID(ctx context.Context, id string) (*domain.Report, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Report); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockReportRepo) ListPending(ctx context.Context) ([]*domain.Report, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Report), args.Error(1)
}
func (m *MockReportRepo) UpdateStatus(ctx context.Context, id string, status domain.ReportStatus) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *MockReportRepo) SaveResult(ctx context.Context, id string, result interface{}) error {
	return m.Called(ctx, id, result).Error(0)
}
func (m *MockReportRepo) SaveError(ctx context.Context, id string, errMsg string) error {
	return m.Called(ctx, id, errMsg).Error(0)
}

// — mock: EventRepository —

type MockEventRepo struct{ mock.Mock }

func (m *MockEventRepo) GetMonthlySales(ctx context.Context, from, to time.Time) ([]usecase.MonthlySalesRow, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]usecase.MonthlySalesRow), args.Error(1)
}
func (m *MockEventRepo) GetSalesReport(ctx context.Context, from, to time.Time) ([]usecase.SalesReportRow, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]usecase.SalesReportRow), args.Error(1)
}
func (m *MockEventRepo) GetWriteOffReport(ctx context.Context, from, to time.Time) ([]usecase.WriteOffReportRow, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]usecase.WriteOffReportRow), args.Error(1)
}
func (m *MockEventRepo) GetMonthlyWriteOffs(ctx context.Context, from, to time.Time) ([]usecase.MonthlyWriteOffRow, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]usecase.MonthlyWriteOffRow), args.Error(1)
}
func (m *MockEventRepo) GetMonthlyReceived(ctx context.Context, from, to time.Time) ([]usecase.MonthlyReceivedRow, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]usecase.MonthlyReceivedRow), args.Error(1)
}
func (m *MockEventRepo) GetDistinctProducts(ctx context.Context) ([]usecase.ProductInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]usecase.ProductInfo), args.Error(1)
}

// — mock: InventoryClient —

type MockInventoryClient struct{ mock.Mock }

func (m *MockInventoryClient) GetStock(ctx context.Context, productID string) (int, int, error) {
	args := m.Called(ctx, productID)
	return args.Int(0), args.Int(1), args.Error(2)
}
func (m *MockInventoryClient) ListExpiringBatches(ctx context.Context, daysAhead int) ([]usecase.ExpiringBatch, error) {
	args := m.Called(ctx, daysAhead)
	return args.Get(0).([]usecase.ExpiringBatch), args.Error(1)
}

// noopSeasonalFn returns 1.0 for all inputs (neutral seasonal coefficient).
func noopSeasonalFn(_ string, _ int) float64 { return 1.0 }

// — tests: CreateSalesReport —

func TestCreateSalesReport(t *testing.T) {
	tests := []struct {
		name        string
		period      string
		setupMocks  func(repo *MockReportRepo)
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid period creates pending report",
			period: "MONTH",
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Report) bool {
					return r.Type == domain.TypeSalesReport &&
						r.Status == domain.StatusPending &&
						r.Params["period"] == "MONTH" &&
						r.ID != ""
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "half-year period creates pending report",
			period: "HALF_YEAR",
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Report) bool {
					return r.Type == domain.TypeSalesReport && r.Status == domain.StatusPending
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "repository error propagates",
			period: "YEAR",
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.Anything).
					Return(errors.New("db connection failed"))
			},
			wantErr:     true,
			errContains: "create sales report",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			id, err := uc.CreateSalesReport(context.Background(), tc.period)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Empty(t, id)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: GetSalesReport —

func TestGetSalesReport(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(repo *MockReportRepo)
		wantErr bool
	}{
		{
			name: "returns report with pending status",
			id:   "report-001",
			setup: func(repo *MockReportRepo) {
				repo.On("GetByID", mock.Anything, "report-001").
					Return(&domain.Report{
						ID:     "report-001",
						Type:   domain.TypeSalesReport,
						Status: domain.StatusPending,
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "returns report with ready status",
			id:   "report-002",
			setup: func(repo *MockReportRepo) {
				repo.On("GetByID", mock.Anything, "report-002").
					Return(&domain.Report{
						ID:     "report-002",
						Type:   domain.TypeSalesReport,
						Status: domain.StatusReady,
					}, nil)
			},
			wantErr: false,
		},
		{
			name: "not found returns error",
			id:   "bad-id",
			setup: func(repo *MockReportRepo) {
				repo.On("GetByID", mock.Anything, "bad-id").
					Return(nil, errors.New("report not found"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setup(repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			report, err := uc.GetSalesReport(context.Background(), tc.id)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, report)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, report)
			assert.Equal(t, tc.id, report.ID)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: CreateForecast —

func TestCreateForecast(t *testing.T) {
	tests := []struct {
		name           string
		lookbackMonths int
		setupMocks     func(repo *MockReportRepo)
		wantErr        bool
		errContains    string
	}{
		{
			name:           "1-month lookback creates pending report",
			lookbackMonths: 1,
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Report) bool {
					lb, _ := r.Params["lookback_months"].(int)
					return r.Type == domain.TypeForecast &&
						r.Status == domain.StatusPending &&
						lb == 1 &&
						r.ID != ""
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "6-month lookback creates pending report",
			lookbackMonths: 6,
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Report) bool {
					lb, _ := r.Params["lookback_months"].(int)
					return r.Type == domain.TypeForecast &&
						r.Status == domain.StatusPending &&
						lb == 6
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "12-month lookback creates pending report",
			lookbackMonths: 12,
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.MatchedBy(func(r *domain.Report) bool {
					lb, _ := r.Params["lookback_months"].(int)
					return r.Type == domain.TypeForecast &&
						r.Status == domain.StatusPending &&
						lb == 12
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:           "repository error propagates",
			lookbackMonths: 3,
			setupMocks: func(repo *MockReportRepo) {
				repo.On("Create", mock.Anything, mock.Anything).
					Return(errors.New("db error"))
			},
			wantErr:     true,
			errContains: "create forecast",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			id, err := uc.CreateForecast(context.Background(), tc.lookbackMonths)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				assert.Empty(t, id)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, id)
			repo.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (sales) —

func TestProcessReport_SalesReport(t *testing.T) {
	tests := []struct {
		name       string
		report     *domain.Report
		setupMocks func(events *MockEventRepo, reports *MockReportRepo)
		wantErr    bool
	}{
		{
			name: "processes sales report with items",
			report: &domain.Report{
				ID:     "report-sales-1",
				Type:   domain.TypeSalesReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetSalesReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.SalesReportRow{
						{ProductID: "p1", ProductName: "Aspirin", TotalQty: 100, Revenue: 500.0, AvgPrice: 5.0},
						{ProductID: "p2", ProductName: "Ibuprofen", TotalQty: 50, Revenue: 300.0, AvgPrice: 6.0},
					}, nil)
				reports.On("SaveResult", mock.Anything, "report-sales-1", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.SalesReportResult)
					return ok && len(result.Items) == 2 &&
						result.TotalRevenue == 800.0 &&
						result.TotalQuantity == 150
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "processes sales report with empty data",
			report: &domain.Report{
				ID:     "report-sales-2",
				Type:   domain.TypeSalesReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "YEAR"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetSalesReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.SalesReportRow{}, nil)
				reports.On("SaveResult", mock.Anything, "report-sales-2", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.SalesReportResult)
					return ok && len(result.Items) == 0 && result.TotalRevenue == 0
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "event repository error propagates",
			report: &domain.Report{
				ID:     "report-sales-3",
				Type:   domain.TypeSalesReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetSalesReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.SalesReportRow{}, errors.New("clickhouse error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(events, repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			err := uc.ProcessReport(context.Background(), tc.report)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			repo.AssertExpectations(t)
			events.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (write-off) —

func TestProcessReport_WriteOffReport(t *testing.T) {
	tests := []struct {
		name       string
		report     *domain.Report
		setupMocks func(events *MockEventRepo, reports *MockReportRepo)
		wantErr    bool
	}{
		{
			name: "processes write-off report with items",
			report: &domain.Report{
				ID:     "report-wo-1",
				Type:   domain.TypeWriteOffReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetWriteOffReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.WriteOffReportRow{
						{ProductID: "p1", ProductName: "Aspirin", TotalQty: 10},
						{ProductID: "p2", ProductName: "Ibuprofen", TotalQty: 5},
					}, nil)
				reports.On("SaveResult", mock.Anything, "report-wo-1", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.WriteOffReportResult)
					return ok && len(result.Items) == 2 && result.TotalWrittenOff == 15
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "processes write-off report with empty data",
			report: &domain.Report{
				ID:     "report-wo-2",
				Type:   domain.TypeWriteOffReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "YEAR"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetWriteOffReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.WriteOffReportRow{}, nil)
				reports.On("SaveResult", mock.Anything, "report-wo-2", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.WriteOffReportResult)
					return ok && len(result.Items) == 0 && result.TotalWrittenOff == 0
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "event repository error propagates",
			report: &domain.Report{
				ID:     "report-wo-3",
				Type:   domain.TypeWriteOffReport,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetWriteOffReport", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.WriteOffReportRow{}, errors.New("clickhouse unavailable"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(events, repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			err := uc.ProcessReport(context.Background(), tc.report)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			repo.AssertExpectations(t)
			events.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport (waste analysis) —

func TestProcessReport_WasteAnalysis(t *testing.T) {
	baseMonth := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		report     *domain.Report
		setupMocks func(events *MockEventRepo, reports *MockReportRepo)
		wantErr    bool
		validate   func(t *testing.T)
	}{
		{
			name: "processes waste analysis with high waste ratio",
			report: &domain.Report{
				ID:     "report-wa-1",
				Type:   domain.TypeWasteAnalysis,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetMonthlyReceived", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyReceivedRow{
						{ProductID: "p1", Month: baseMonth, Quantity: 100},
					}, nil)
				events.On("GetMonthlyWriteOffs", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyWriteOffRow{
						{ProductID: "p1", Month: baseMonth, Quantity: 40},
					}, nil)
				events.On("GetDistinctProducts", mock.Anything).
					Return([]usecase.ProductInfo{
						{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
					}, nil)
				reports.On("SaveResult", mock.Anything, "report-wa-1", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.WasteAnalysisResult)
					if !ok || len(result.Items) != 1 {
						return false
					}
					item := result.Items[0]
					// waste ratio = 40/100 = 0.40 → high waste
					return item.ProductID == "p1" &&
						item.Received == 100 &&
						item.WrittenOff == 40 &&
						item.WasteRatio == 0.4 &&
						item.Recommendation != ""
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "skips products with zero received quantity",
			report: &domain.Report{
				ID:     "report-wa-2",
				Type:   domain.TypeWasteAnalysis,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetMonthlyReceived", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyReceivedRow{}, nil)
				events.On("GetMonthlyWriteOffs", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyWriteOffRow{
						{ProductID: "p1", Month: baseMonth, Quantity: 5},
					}, nil)
				events.On("GetDistinctProducts", mock.Anything).
					Return([]usecase.ProductInfo{
						{ProductID: "p1", ProductName: "Aspirin", TherapeuticGroup: "painkiller"},
					}, nil)
				reports.On("SaveResult", mock.Anything, "report-wa-2", mock.MatchedBy(func(r interface{}) bool {
					result, ok := r.(*domain.WasteAnalysisResult)
					return ok && len(result.Items) == 0
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "get monthly received error propagates",
			report: &domain.Report{
				ID:     "report-wa-3",
				Type:   domain.TypeWasteAnalysis,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetMonthlyReceived", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyReceivedRow{}, errors.New("clickhouse error"))
			},
			wantErr: true,
		},
		{
			name: "get monthly write-offs error propagates",
			report: &domain.Report{
				ID:     "report-wa-4",
				Type:   domain.TypeWasteAnalysis,
				Status: domain.StatusProcessing,
				Params: map[string]interface{}{"period": "MONTH"},
			},
			setupMocks: func(events *MockEventRepo, reports *MockReportRepo) {
				events.On("GetMonthlyReceived", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyReceivedRow{}, nil)
				events.On("GetMonthlyWriteOffs", mock.Anything, mock.Anything, mock.Anything).
					Return([]usecase.MonthlyWriteOffRow{}, errors.New("write-off query failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockReportRepo)
			events := new(MockEventRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(events, repo)

			uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
			err := uc.ProcessReport(context.Background(), tc.report)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			repo.AssertExpectations(t)
			events.AssertExpectations(t)
		})
	}
}

// — tests: ProcessReport unknown type —

func TestProcessReport_UnknownType(t *testing.T) {
	repo := new(MockReportRepo)
	events := new(MockEventRepo)
	inv := new(MockInventoryClient)

	uc := usecase.NewAnalyticsUseCase(repo, events, inv, noopSeasonalFn)
	err := uc.ProcessReport(context.Background(), &domain.Report{
		ID:     "report-x",
		Type:   domain.ReportType("unknown_type"),
		Status: domain.StatusProcessing,
		Params: map[string]interface{}{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown report type")
}
