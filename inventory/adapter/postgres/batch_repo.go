package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
)

type BatchRepository struct {
	db *sql.DB
}

func NewBatchRepository(db *sql.DB) *BatchRepository {
	return &BatchRepository{db: db}
}

const selectBatch = `
	SELECT id, product_id, series_number, expires_at, received_at,
	       quantity, reserved, purchase_price, retail_price, status
	FROM batches`

func (r *BatchRepository) Create(ctx context.Context, b *domain.Batch) error {
	const query = `
		INSERT INTO batches
			(id, product_id, series_number, expires_at, received_at,
			 quantity, reserved, purchase_price, retail_price, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`

	_, err := r.db.ExecContext(ctx, query,
		b.ID, b.ProductID, b.SeriesNumber, b.ExpiresAt, b.ReceivedAt,
		b.Quantity, b.Reserved, b.PurchasePrice, b.RetailPrice, string(b.Status),
	)
	if err != nil {
		return fmt.Errorf("postgres.BatchRepository.Create: %w", err)
	}
	return nil
}

func (r *BatchRepository) GetByID(ctx context.Context, id string) (*domain.Batch, error) {
	row := r.db.QueryRowContext(ctx, selectBatch+` WHERE id=$1`, id)
	b, err := scanBatch(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, usecase.ErrBatchNotFound
		}
		return nil, fmt.Errorf("postgres.BatchRepository.GetByID: %w", err)
	}
	return b, nil
}

func (r *BatchRepository) ListByProduct(ctx context.Context, productID string) ([]*domain.Batch, error) {
	rows, err := r.db.QueryContext(ctx,
		selectBatch+` WHERE product_id=$1 AND status != 'written_off' ORDER BY expires_at`,
		productID)
	if err != nil {
		return nil, fmt.Errorf("postgres.BatchRepository.ListByProduct: %w", err)
	}
	defer rows.Close()
	return scanBatches(rows)
}

func (r *BatchRepository) ListExpiring(ctx context.Context, daysAhead int) ([]*domain.Batch, error) {
	threshold := time.Now().AddDate(0, 0, daysAhead)
	rows, err := r.db.QueryContext(ctx,
		selectBatch+` WHERE expires_at <= $1 AND status NOT IN ('expired','written_off') ORDER BY expires_at`,
		threshold)
	if err != nil {
		return nil, fmt.Errorf("postgres.BatchRepository.ListExpiring: %w", err)
	}
	defer rows.Close()
	return scanBatches(rows)
}

func (r *BatchRepository) ListExpired(ctx context.Context) ([]*domain.Batch, error) {
	rows, err := r.db.QueryContext(ctx,
		selectBatch+` WHERE expires_at < $1 AND status != 'written_off'`,
		time.Now())
	if err != nil {
		return nil, fmt.Errorf("postgres.BatchRepository.ListExpired: %w", err)
	}
	defer rows.Close()
	return scanBatches(rows)
}

func (r *BatchRepository) UpdateStatus(ctx context.Context, id string, status domain.BatchStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE batches SET status=$2 WHERE id=$1`, id, string(status))
	if err != nil {
		return fmt.Errorf("postgres.BatchRepository.UpdateStatus: %w", err)
	}
	return nil
}

func (r *BatchRepository) Save(ctx context.Context, b *domain.Batch) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE batches SET quantity=$2, reserved=$3, status=$4 WHERE id=$1`,
		b.ID, b.Quantity, b.Reserved, string(b.Status))
	if err != nil {
		return fmt.Errorf("postgres.BatchRepository.Save: %w", err)
	}
	return nil
}

func scanBatch(row *sql.Row) (*domain.Batch, error) {
	b := &domain.Batch{}
	var status string
	err := row.Scan(&b.ID, &b.ProductID, &b.SeriesNumber, &b.ExpiresAt, &b.ReceivedAt,
		&b.Quantity, &b.Reserved, &b.PurchasePrice, &b.RetailPrice, &status)
	if err != nil {
		return nil, err
	}
	b.Status = domain.BatchStatus(status)
	return b, nil
}

func scanBatches(rows *sql.Rows) ([]*domain.Batch, error) {
	var result []*domain.Batch
	for rows.Next() {
		b := &domain.Batch{}
		var status string
		if err := rows.Scan(&b.ID, &b.ProductID, &b.SeriesNumber, &b.ExpiresAt, &b.ReceivedAt,
			&b.Quantity, &b.Reserved, &b.PurchasePrice, &b.RetailPrice, &status); err != nil {
			return nil, fmt.Errorf("postgres.BatchRepository scan: %w", err)
		}
		b.Status = domain.BatchStatus(status)
		result = append(result, b)
	}
	return result, nil
}
