package usecase

import (
	"context"

	"pharmacy/sales/domain"
)

// SaleRepository — порт для хранения продаж.
type SaleRepository interface {
	Create(ctx context.Context, sale *domain.Sale) error
	GetByID(ctx context.Context, id string) (*domain.Sale, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error)
}

// InventoryClient — порт для взаимодействия с Inventory сервисом.
type InventoryClient interface {
	DeductStock(ctx context.Context, productID string, qty int, orderID string) (batchID string, retailPrice float64, err error)
}

type CreateSaleItemInput struct {
	ProductID string
	Quantity  int
}

type CreateSaleInput struct {
	Items []CreateSaleItemInput
}
