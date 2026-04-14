package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pharmacy/sales/domain"
)

type SaleRepository struct {
	db *sql.DB
}

func NewSaleRepository(db *sql.DB) *SaleRepository {
	return &SaleRepository{db: db}
}

func (r *SaleRepository) Create(ctx context.Context, sale *domain.Sale) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres.SaleRepository.Create begin tx: %w", err)
	}
	defer tx.Rollback()

	const insertSale = `
		INSERT INTO sales (id, total_amount, sold_at)
		VALUES ($1, $2, $3)`

	if _, err := tx.ExecContext(ctx, insertSale, sale.ID, sale.TotalAmount, sale.SoldAt); err != nil {
		return fmt.Errorf("postgres.SaleRepository.Create sale: %w", err)
	}

	const insertItem = `
		INSERT INTO sale_items (id, sale_id, product_id, quantity, price_per_unit, total_price)
		VALUES ($1, $2, $3, $4, $5, $6)`

	for _, item := range sale.Items {
		if _, err := tx.ExecContext(ctx, insertItem,
			item.ID, item.SaleID, item.ProductID,
			item.Quantity, item.PricePerUnit, item.TotalPrice,
		); err != nil {
			return fmt.Errorf("postgres.SaleRepository.Create item: %w", err)
		}
	}

	return tx.Commit()
}

func (r *SaleRepository) GetByID(ctx context.Context, id string) (*domain.Sale, error) {
	const saleQuery = `SELECT id, total_amount, sold_at FROM sales WHERE id = $1`

	sale := &domain.Sale{}
	err := r.db.QueryRowContext(ctx, saleQuery, id).
		Scan(&sale.ID, &sale.TotalAmount, &sale.SoldAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSaleNotFound
		}
		return nil, fmt.Errorf("postgres.SaleRepository.GetByID: %w", err)
	}

	const itemsQuery = `
		SELECT id, sale_id, product_id, quantity, price_per_unit, total_price
		FROM sale_items WHERE sale_id = $1 ORDER BY id`

	rows, err := r.db.QueryContext(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("postgres.SaleRepository.GetByID items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.SaleItem
		if err := rows.Scan(&item.ID, &item.SaleID, &item.ProductID,
			&item.Quantity, &item.PricePerUnit, &item.TotalPrice); err != nil {
			return nil, fmt.Errorf("postgres.SaleRepository.GetByID scan item: %w", err)
		}
		sale.Items = append(sale.Items, item)
	}

	return sale, nil
}

func (r *SaleRepository) List(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error) {
	offset := (page - 1) * pageSize

	const countQuery = `SELECT COUNT(*) FROM sales`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres.SaleRepository.List count: %w", err)
	}

	const salesQuery = `
		SELECT id, total_amount, sold_at
		FROM sales ORDER BY sold_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, salesQuery, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres.SaleRepository.List: %w", err)
	}
	defer rows.Close()

	var sales []*domain.Sale
	for rows.Next() {
		s := &domain.Sale{}
		if err := rows.Scan(&s.ID, &s.TotalAmount, &s.SoldAt); err != nil {
			return nil, 0, fmt.Errorf("postgres.SaleRepository.List scan: %w", err)
		}
		sales = append(sales, s)
	}

	// Load items for each sale
	for _, sale := range sales {
		const itemsQuery = `
			SELECT id, sale_id, product_id, quantity, price_per_unit, total_price
			FROM sale_items WHERE sale_id = $1 ORDER BY id`

		itemRows, err := r.db.QueryContext(ctx, itemsQuery, sale.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres.SaleRepository.List items: %w", err)
		}

		for itemRows.Next() {
			var item domain.SaleItem
			if err := itemRows.Scan(&item.ID, &item.SaleID, &item.ProductID,
				&item.Quantity, &item.PricePerUnit, &item.TotalPrice); err != nil {
				itemRows.Close()
				return nil, 0, fmt.Errorf("postgres.SaleRepository.List scan item: %w", err)
			}
			sale.Items = append(sale.Items, item)
		}
		itemRows.Close()
	}

	return sales, total, nil
}
