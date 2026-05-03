package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"pharmacy/analytics/domain"
	usecase "pharmacy/analytics/domain/use_case"
)

// ReportRepo implements usecase.ReportRepository against PostgreSQL.
type ReportRepo struct {
	db *sql.DB
}

// NewReportRepo creates a new ReportRepo.
func NewReportRepo(db *sql.DB) *ReportRepo {
	return &ReportRepo{db: db}
}

// Create persists a new report.
func (r *ReportRepo) Create(ctx context.Context, report *domain.Report) error {
	paramsJSON, err := json.Marshal(report.Params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO reports (id, type, status, params, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		report.ID,
		string(report.Type),
		string(report.Status),
		paramsJSON,
		report.CreatedAt,
	)
	return err
}

// GetByID retrieves a report by ID.
func (r *ReportRepo) GetByID(ctx context.Context, id string) (*domain.Report, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, type, status, params, result, created_at, completed_at, error
		 FROM reports WHERE id = $1`,
		id,
	)
	return scanReport(row)
}

// ListPending returns all reports with status=pending.
func (r *ReportRepo) ListPending(ctx context.Context) ([]*domain.Report, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, type, status, params, result, created_at, completed_at, error
		 FROM reports WHERE status = $1 ORDER BY created_at`,
		string(domain.StatusPending),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*domain.Report
	for rows.Next() {
		report, err := scanReportRow(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

// UpdateStatus updates a report's status.
func (r *ReportRepo) UpdateStatus(ctx context.Context, id string, status domain.ReportStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE reports SET status = $1 WHERE id = $2`,
		string(status), id,
	)
	return err
}

// SaveResult sets status=ready, completed_at=now, result=result.
func (r *ReportRepo) SaveResult(ctx context.Context, id string, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx,
		`UPDATE reports SET status = $1, result = $2, completed_at = $3 WHERE id = $4`,
		string(domain.StatusReady), resultJSON, now, id,
	)
	return err
}

// SaveError sets status=failed, completed_at=now, error=errMsg.
func (r *ReportRepo) SaveError(ctx context.Context, id string, errMsg string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`UPDATE reports SET status = $1, error = $2, completed_at = $3 WHERE id = $4`,
		string(domain.StatusFailed), errMsg, now, id,
	)
	return err
}

// — helpers —

func scanReport(row *sql.Row) (*domain.Report, error) {
	var (
		id          string
		reportType  string
		status      string
		paramsJSON  []byte
		resultJSON  []byte
		createdAt   time.Time
		completedAt sql.NullTime
		errMsg      string
	)
	if err := row.Scan(&id, &reportType, &status, &paramsJSON, &resultJSON, &createdAt, &completedAt, &errMsg); err != nil {
		return nil, err
	}
	return buildReport(id, reportType, status, paramsJSON, resultJSON, createdAt, completedAt, errMsg)
}

func scanReportRow(rows *sql.Rows) (*domain.Report, error) {
	var (
		id          string
		reportType  string
		status      string
		paramsJSON  []byte
		resultJSON  []byte
		createdAt   time.Time
		completedAt sql.NullTime
		errMsg      string
	)
	if err := rows.Scan(&id, &reportType, &status, &paramsJSON, &resultJSON, &createdAt, &completedAt, &errMsg); err != nil {
		return nil, err
	}
	return buildReport(id, reportType, status, paramsJSON, resultJSON, createdAt, completedAt, errMsg)
}

func buildReport(
	id, reportType, status string,
	paramsJSON, resultJSON []byte,
	createdAt time.Time,
	completedAt sql.NullTime,
	errMsg string,
) (*domain.Report, error) {
	var params map[string]interface{}
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &params); err != nil {
			return nil, fmt.Errorf("unmarshal params: %w", err)
		}
	}

	var result interface{}
	if len(resultJSON) > 0 {
		result, _ = unmarshalResult(domain.ReportType(reportType), resultJSON)
	}

	report := &domain.Report{
		ID:        id,
		Type:      domain.ReportType(reportType),
		Status:    domain.ReportStatus(status),
		Params:    params,
		Result:    result,
		CreatedAt: createdAt,
		Error:     errMsg,
	}
	if completedAt.Valid {
		t := completedAt.Time
		report.CompletedAt = &t
	}
	return report, nil
}

// unmarshalResult deserializes result JSON into the correct typed struct.
func unmarshalResult(reportType domain.ReportType, data []byte) (interface{}, error) {
	switch reportType {
	case domain.TypeSalesReport:
		var v domain.SalesReportResult
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case domain.TypeWriteOffReport:
		var v domain.WriteOffReportResult
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case domain.TypeForecast:
		var v domain.ForecastResult
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	case domain.TypeWasteAnalysis:
		var v domain.WasteAnalysisResult
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return &v, nil
	default:
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	}
}

// Ensure interface compliance at compile time.
var _ usecase.ReportRepository = (*ReportRepo)(nil)
