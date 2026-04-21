package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	usecase "pharmacy/analytics/domain/use_case"
)

// EventRepo implements usecase.EventRepository against ClickHouse.
type EventRepo struct {
	conn driver.Conn
}

// NewEventRepo opens a ClickHouse connection using the provided DSN.
func NewEventRepo(dsn string) (*EventRepo, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	return &EventRepo{conn: conn}, nil
}

// — EventRepository interface implementation —

// GetMonthlySales returns monthly aggregated sales from ClickHouse.
func (r *EventRepo) GetMonthlySales(ctx context.Context, from, to time.Time) ([]usecase.MonthlySalesRow, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			product_id,
			any(product_name)      AS product_name,
			any(therapeutic_group) AS therapeutic_group,
			toStartOfMonth(sold_at) AS month,
			sum(quantity)          AS quantity,
			sum(total_price)       AS revenue
		FROM sales_events
		WHERE sold_at >= ? AND sold_at < ?
		GROUP BY product_id, toStartOfMonth(sold_at)
		ORDER BY product_id, month
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetMonthlySales query: %w", err)
	}
	defer rows.Close()

	var result []usecase.MonthlySalesRow
	for rows.Next() {
		var row usecase.MonthlySalesRow
		var qty uint64
		if err := rows.Scan(
			&row.ProductID,
			&row.ProductName,
			&row.TherapeuticGroup,
			&row.Month,
			&qty,
			&row.Revenue,
		); err != nil {
			return nil, fmt.Errorf("GetMonthlySales scan: %w", err)
		}

		row.Quantity = int(qty)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetSalesReport returns per-product aggregated sales for a date range.
func (r *EventRepo) GetSalesReport(ctx context.Context, from, to time.Time) ([]usecase.SalesReportRow, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			product_id,
			any(product_name) AS product_name,
			sum(quantity)     AS total_qty,
			sum(total_price)  AS revenue,
			avg(price_per_unit) AS avg_price
		FROM sales_events
		WHERE sold_at >= ? AND sold_at < ?
		GROUP BY product_id
		ORDER BY revenue DESC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetSalesReport query: %w", err)
	}
	defer rows.Close()

	var result []usecase.SalesReportRow
	for rows.Next() {
		var row usecase.SalesReportRow
		var qty uint64
		if err := rows.Scan(
			&row.ProductID,
			&row.ProductName,
			&qty,
			&row.Revenue,
			&row.AvgPrice,
		); err != nil {
			return nil, fmt.Errorf("GetSalesReport scan: %w", err)
		}
		row.TotalQty = int(qty)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetWriteOffReport returns per-product aggregated write-offs for a date range.
func (r *EventRepo) GetWriteOffReport(ctx context.Context, from, to time.Time) ([]usecase.WriteOffReportRow, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			product_id,
			any(product_name) AS product_name,
			sum(quantity)     AS total_qty
		FROM write_off_events
		WHERE written_off_at >= ? AND written_off_at < ?
		GROUP BY product_id
		ORDER BY total_qty DESC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetWriteOffReport query: %w", err)
	}
	defer rows.Close()

	var result []usecase.WriteOffReportRow
	for rows.Next() {
		var row usecase.WriteOffReportRow
		var qty uint64
		if err := rows.Scan(
			&row.ProductID,
			&row.ProductName,
			&qty,
		); err != nil {
			return nil, fmt.Errorf("GetWriteOffReport scan: %w", err)
		}
		row.TotalQty = int(qty)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetMonthlyWriteOffs returns monthly aggregated write-offs for a date range.
func (r *EventRepo) GetMonthlyWriteOffs(ctx context.Context, from, to time.Time) ([]usecase.MonthlyWriteOffRow, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			product_id,
			toStartOfMonth(written_off_at) AS month,
			sum(quantity) AS quantity
		FROM write_off_events
		WHERE written_off_at >= ? AND written_off_at < ?
		GROUP BY product_id, toStartOfMonth(written_off_at)
		ORDER BY product_id, month
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetMonthlyWriteOffs query: %w", err)
	}
	defer rows.Close()

	var result []usecase.MonthlyWriteOffRow
	for rows.Next() {
		var row usecase.MonthlyWriteOffRow
		var qty uint64
		if err := rows.Scan(&row.ProductID, &row.Month, &qty); err != nil {
			return nil, fmt.Errorf("GetMonthlyWriteOffs scan: %w", err)
		}
		row.Quantity = int(qty)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetMonthlyReceived returns monthly aggregated received quantities for a date range.
func (r *EventRepo) GetMonthlyReceived(ctx context.Context, from, to time.Time) ([]usecase.MonthlyReceivedRow, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT
			product_id,
			toStartOfMonth(received_at) AS month,
			sum(quantity) AS quantity
		FROM received_events
		WHERE received_at >= ? AND received_at < ?
		GROUP BY product_id, toStartOfMonth(received_at)
		ORDER BY product_id, month
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("GetMonthlyReceived query: %w", err)
	}
	defer rows.Close()

	var result []usecase.MonthlyReceivedRow
	for rows.Next() {
		var row usecase.MonthlyReceivedRow
		var qty uint64
		if err := rows.Scan(&row.ProductID, &row.Month, &qty); err != nil {
			return nil, fmt.Errorf("GetMonthlyReceived scan: %w", err)
		}

		row.Quantity = int(qty)
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetDistinctProducts returns all distinct products that appear in any event table.
func (r *EventRepo) GetDistinctProducts(ctx context.Context) ([]usecase.ProductInfo, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT product_id, any(product_name) AS product_name, any(therapeutic_group) AS therapeutic_group
		FROM (
			SELECT product_id, product_name, therapeutic_group FROM sales_events
			UNION ALL
			SELECT product_id, product_name, therapeutic_group FROM write_off_events
			UNION ALL
			SELECT product_id, product_name, therapeutic_group FROM received_events
		)
		GROUP BY product_id
		ORDER BY product_id
	`)
	if err != nil {
		return nil, fmt.Errorf("GetDistinctProducts query: %w", err)
	}
	defer rows.Close()

	var result []usecase.ProductInfo
	for rows.Next() {
		var p usecase.ProductInfo
		if err := rows.Scan(&p.ProductID, &p.ProductName, &p.TherapeuticGroup); err != nil {
			return nil, fmt.Errorf("GetDistinctProducts scan: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// RunMigrations executes ClickHouse DDL to create event tables.
func (r *EventRepo) RunMigrations(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sales_events (
			product_id        String,
			product_name      String,
			therapeutic_group String,
			quantity          UInt32,
			price_per_unit    Float64,
			total_price       Float64,
			sold_at           DateTime
		) ENGINE = MergeTree()
		ORDER BY (product_id, sold_at)`,

		`CREATE TABLE IF NOT EXISTS write_off_events (
			product_id        String,
			product_name      String,
			therapeutic_group String,
			batch_id          String,
			quantity          UInt32,
			expires_at        DateTime,
			written_off_at    DateTime
		) ENGINE = MergeTree()
		ORDER BY (product_id, written_off_at)`,

		`CREATE TABLE IF NOT EXISTS received_events (
			product_id        String,
			product_name      String,
			therapeutic_group String,
			batch_id          String,
			quantity          UInt32,
			retail_price      Float64,
			expires_at        DateTime,
			received_at       DateTime
		) ENGINE = MergeTree()
		ORDER BY (product_id, received_at)`,
	}
	for _, stmt := range statements {
		if err := r.conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("clickhouse migration: %w", err)
		}
	}
	return nil
}

// — Insert methods for Kafka consumer —

// SaleEvent represents a sale.completed Kafka event.
type SaleEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	Quantity         int       `json:"quantity"`
	PricePerUnit     float64   `json:"price_per_unit"`
	TotalPrice       float64   `json:"total_price"`
	SoldAt           time.Time `json:"sold_at"`
}

// WriteOffEvent represents an inventory.written_off Kafka event.
type WriteOffEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	BatchID          string    `json:"batch_id"`
	Quantity         int       `json:"quantity"`
	ExpiresAt        time.Time `json:"expires_at"`
	WrittenOffAt     time.Time `json:"written_off_at"`
}

// ReceivedEvent represents an inventory.received Kafka event.
type ReceivedEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	BatchID          string    `json:"batch_id"`
	Quantity         int       `json:"quantity"`
	RetailPrice      float64   `json:"retail_price"`
	ExpiresAt        time.Time `json:"expires_at"`
	ReceivedAt       time.Time `json:"received_at"`
}

// InsertSaleEvent inserts a sale event into ClickHouse.
func (r *EventRepo) InsertSaleEvent(ctx context.Context, e SaleEvent) error {
	return r.conn.Exec(ctx,
		`INSERT INTO sales_events (product_id, product_name, therapeutic_group, quantity, price_per_unit, total_price, sold_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ProductID, e.ProductName, e.TherapeuticGroup,
		uint32(e.Quantity), e.PricePerUnit, e.TotalPrice, e.SoldAt,
	)
}

// InsertWriteOffEvent inserts a write-off event into ClickHouse.
func (r *EventRepo) InsertWriteOffEvent(ctx context.Context, e WriteOffEvent) error {
	return r.conn.Exec(ctx,
		`INSERT INTO write_off_events (product_id, product_name, therapeutic_group, batch_id, quantity, expires_at, written_off_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ProductID, e.ProductName, e.TherapeuticGroup,
		e.BatchID, uint32(e.Quantity), e.ExpiresAt, e.WrittenOffAt,
	)
}

// InsertReceivedEvent inserts a received event into ClickHouse.
func (r *EventRepo) InsertReceivedEvent(ctx context.Context, e ReceivedEvent) error {
	return r.conn.Exec(ctx,
		`INSERT INTO received_events (product_id, product_name, therapeutic_group, batch_id, quantity, retail_price, expires_at, received_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ProductID, e.ProductName, e.TherapeuticGroup,
		e.BatchID, uint32(e.Quantity), e.RetailPrice, e.ExpiresAt, e.ReceivedAt,
	)
}
