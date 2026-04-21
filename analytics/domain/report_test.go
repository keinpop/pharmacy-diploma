package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/analytics/domain"
)

// — tests: ReportStatus constants —

func TestReportStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   domain.ReportStatus
		expected string
	}{
		{
			name:     "StatusPending has correct string value",
			status:   domain.StatusPending,
			expected: "pending",
		},
		{
			name:     "StatusProcessing has correct string value",
			status:   domain.StatusProcessing,
			expected: "processing",
		},
		{
			name:     "StatusReady has correct string value",
			status:   domain.StatusReady,
			expected: "ready",
		},
		{
			name:     "StatusFailed has correct string value",
			status:   domain.StatusFailed,
			expected: "failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, domain.ReportStatus(tc.expected), tc.status)
			assert.Equal(t, tc.expected, string(tc.status))
		})
	}
}

// — tests: all status constants are distinct —

func TestReportStatusConstants_AreDistinct(t *testing.T) {
	statuses := []domain.ReportStatus{
		domain.StatusPending,
		domain.StatusProcessing,
		domain.StatusReady,
		domain.StatusFailed,
	}

	seen := make(map[domain.ReportStatus]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate status constant: %q", s)
		seen[s] = true
	}
}

// — tests: ReportType constants —

func TestReportTypeConstants(t *testing.T) {
	tests := []struct {
		name       string
		reportType domain.ReportType
		expected   string
	}{
		{
			name:       "TypeSalesReport has correct string value",
			reportType: domain.TypeSalesReport,
			expected:   "sales_report",
		},
		{
			name:       "TypeWriteOffReport has correct string value",
			reportType: domain.TypeWriteOffReport,
			expected:   "write_off_report",
		},
		{
			name:       "TypeForecast has correct string value",
			reportType: domain.TypeForecast,
			expected:   "forecast",
		},
		{
			name:       "TypeWasteAnalysis has correct string value",
			reportType: domain.TypeWasteAnalysis,
			expected:   "waste_analysis",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, domain.ReportType(tc.expected), tc.reportType)
			assert.Equal(t, tc.expected, string(tc.reportType))
		})
	}
}

// — tests: all report type constants are distinct —

func TestReportTypeConstants_AreDistinct(t *testing.T) {
	types := []domain.ReportType{
		domain.TypeSalesReport,
		domain.TypeWriteOffReport,
		domain.TypeForecast,
		domain.TypeWasteAnalysis,
	}

	seen := make(map[domain.ReportType]bool)
	for _, rt := range types {
		assert.False(t, seen[rt], "duplicate report type constant: %q", rt)
		seen[rt] = true
	}
}

// — tests: NewReport —

func TestNewReport(t *testing.T) {
	tests := []struct {
		name       string
		reportType domain.ReportType
		params     map[string]interface{}
	}{
		{
			name:       "sales report with period param",
			reportType: domain.TypeSalesReport,
			params:     map[string]interface{}{"period": "MONTH"},
		},
		{
			name:       "forecast report with lookback param",
			reportType: domain.TypeForecast,
			params:     map[string]interface{}{"lookback_months": 6},
		},
		{
			name:       "write-off report with period param",
			reportType: domain.TypeWriteOffReport,
			params:     map[string]interface{}{"period": "HALF_YEAR"},
		},
		{
			name:       "waste analysis report with period param",
			reportType: domain.TypeWasteAnalysis,
			params:     map[string]interface{}{"period": "YEAR"},
		},
		{
			name:       "report with nil params",
			reportType: domain.TypeSalesReport,
			params:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now().UTC()
			report := domain.NewReport(tc.reportType, tc.params)
			after := time.Now().UTC()

			require.NotNil(t, report)
			assert.NotEmpty(t, report.ID, "ID must be generated")
			assert.Equal(t, tc.reportType, report.Type)
			assert.Equal(t, domain.StatusPending, report.Status, "new reports must be pending")
			assert.Equal(t, tc.params, report.Params)
			assert.Nil(t, report.Result)
			assert.Empty(t, report.Error)
			assert.Nil(t, report.CompletedAt)

			assert.True(t, !report.CreatedAt.Before(before),
				"CreatedAt must not be before test start")
			assert.True(t, !report.CreatedAt.After(after),
				"CreatedAt must not be after test end")
		})
	}
}

// — tests: NewReport generates unique IDs —

func TestNewReport_UniqueIDs(t *testing.T) {
	const count = 100
	ids := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		r := domain.NewReport(domain.TypeSalesReport, nil)
		require.NotEmpty(t, r.ID)
		assert.False(t, ids[r.ID], "duplicate ID generated: %s", r.ID)
		ids[r.ID] = true
	}
}
