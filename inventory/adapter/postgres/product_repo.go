package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	const query = `
		INSERT INTO products
        (id, name, trade_name, active_substance, form, dosage,
         category, storage_conditions, unit, therapeutic_group,
         reorder_point, created_at, updated_at)
    	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.TradeName, p.ActiveSubstance,
		p.Form, p.Dosage, string(p.Category), p.StorageConditions,
		p.Unit, p.TherapeuticGroup, p.ReorderPoint,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.ProductRepository.Create: %w", err)
	}
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	const query = `
		SELECT id, name, trade_name, active_substance, form, dosage,
           category, storage_conditions, unit, therapeutic_group,
           reorder_point, created_at, updated_at
    	FROM products WHERE id = $1`

	p := &domain.Product{}
	var cat string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.TradeName, &p.ActiveSubstance,
		&p.Form, &p.Dosage, &p.Category, &p.StorageConditions,
		&p.Unit, &p.TherapeuticGroup, &p.ReorderPoint,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, usecase.ErrProductNotFound
		}
		return nil, fmt.Errorf("postgres.ProductRepository.GetByID: %w", err)
	}
	p.Category = domain.Category(cat)
	return p, nil
}

func (r *ProductRepository) List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	offset := (page - 1) * pageSize
	const query = `
		SELECT id, name, trade_name, active_substance, form, dosage,
           category, storage_conditions, unit, therapeutic_group,
           reorder_point, created_at, updated_at
		FROM products ORDER BY name LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres.ProductRepository.List: %w", err)
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p := &domain.Product{}
		var cat string
		if err := rows.Scan(
			&p.ID, &p.Name, &p.TradeName, &p.ActiveSubstance,
			&p.Form, &p.Dosage, &p.Category, &p.StorageConditions,
			&p.Unit, &p.TherapeuticGroup, &p.ReorderPoint,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("postgres.ProductRepository.List scan: %w", err)
		}
		p.Category = domain.Category(cat)
		products = append(products, p)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres.ProductRepository.List count: %w", err)
	}
	return products, total, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *domain.Product) error {
	const query = `
		UPDATE products SET
        name=$2, trade_name=$3, active_substance=$4, form=$5,
        dosage=$6, category=$7, storage_conditions=$8, unit=$9,
        therapeutic_group=$10, reorder_point=$11, updated_at=$12
    	WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.TradeName, p.ActiveSubstance,
		p.Form, p.Dosage, p.Category, p.StorageConditions,
		p.Unit, p.TherapeuticGroup, p.ReorderPoint, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres.ProductRepository.Update: %w", err)
	}
	return nil
}
