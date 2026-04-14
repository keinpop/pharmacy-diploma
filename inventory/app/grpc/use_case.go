package grpc

import (
	"context"

	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
)

// InventoryUC — интерфейс use-case, используемый хэндлером.
type InventoryUC interface {
	CreateProduct(ctx context.Context, in usecase.CreateProductInput) (*domain.Product, error)
	GetProduct(ctx context.Context, id string) (*domain.Product, error)
	ListProducts(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
	SearchProducts(ctx context.Context, query string, limit int) ([]*domain.Product, error)
	GetAnalogs(ctx context.Context, productID string, limit int) ([]*domain.Product, error)

	ReceiveBatch(ctx context.Context, in usecase.ReceiveBatchInput) (*domain.Batch, error)
	GetStock(ctx context.Context, productID string) (*domain.StockItem, error)
	DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, error)
	ListExpiringBatches(ctx context.Context, daysAhead int) ([]*domain.Batch, error)
	ListLowStock(ctx context.Context) ([]*domain.StockItem, error)
	WriteOffExpired(ctx context.Context) (int, error)
}
