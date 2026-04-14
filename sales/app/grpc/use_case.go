package grpc

import (
	"context"

	"pharmacy/sales/domain"
	usecase "pharmacy/sales/domain/use_case"
)

// SalesUC — интерфейс use-case, используемый хэндлером.
type SalesUC interface {
	CreateSale(ctx context.Context, in usecase.CreateSaleInput) (*domain.Sale, error)
	GetSale(ctx context.Context, id string) (*domain.Sale, error)
	ListSales(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error)
}
