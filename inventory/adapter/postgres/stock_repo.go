package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pharmacy/inventory/domain"
)

type StockRepository struct {
	db *sql.DB
}

func NewStockRepository(db *sql.DB) *StockRepository {
	return &StockRepository{db: db}
}

func (r *StockRepository) GetByProduct(ctx context.Context, productID string) (*domain.StockItem, error) {
	const query = `
		SELECT product_id, total_quantity, reserved, reorder_point
		FROM stock_items WHERE product_id=$1`

	s := &domain.StockItem{}
	err := r.db.QueryRowContext(ctx, query, productID).
		Scan(&s.ProductID, &s.TotalQuantity, &s.Reserved, &s.ReorderPoint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &domain.StockItem{ProductID: productID}, nil
		}
		return nil, fmt.Errorf("postgres.StockRepository.GetByProduct: %w", err)
	}
	return s, nil
}

func (r *StockRepository) Upsert(ctx context.Context, s *domain.StockItem) error {
	const query = `
		INSERT INTO stock_items (product_id, total_quantity, reserved, reorder_point)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (product_id) DO UPDATE
		    SET total_quantity=$2, reserved=$3, reorder_point=$4`

	_, err := r.db.ExecContext(ctx, query,
		s.ProductID, s.TotalQuantity, s.Reserved, s.ReorderPoint)
	if err != nil {
		return fmt.Errorf("postgres.StockRepository.Upsert: %w", err)
	}
	return nil
}

func (r *StockRepository) ListLowStock(ctx context.Context) ([]*domain.StockItem, error) {
	const query = `
		SELECT product_id, total_quantity, reserved, reorder_point
		FROM stock_items WHERE (total_quantity - reserved) <= reorder_point`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("postgres.StockRepository.ListLowStock: %w", err)
	}
	defer rows.Close()

	var result []*domain.StockItem
	for rows.Next() {
		s := &domain.StockItem{}
		if err := rows.Scan(&s.ProductID, &s.TotalQuantity, &s.Reserved, &s.ReorderPoint); err != nil {
			return nil, fmt.Errorf("postgres.StockRepository.ListLowStock scan: %w", err)
		}
		result = append(result, s)
	}
	return result, nil
}
