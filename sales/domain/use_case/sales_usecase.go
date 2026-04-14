package usecase

import (
	"context"

	"pharmacy/sales/domain"
)

type SalesUseCase struct {
	sales     SaleRepository
	inventory InventoryClient
}

func NewSalesUseCase(sales SaleRepository, inventory InventoryClient) *SalesUseCase {
	return &SalesUseCase{sales: sales, inventory: inventory}
}

func (uc *SalesUseCase) CreateSale(ctx context.Context, in CreateSaleInput) (*domain.Sale, error) {
	if len(in.Items) == 0 {
		return nil, domain.ErrEmptyItems
	}

	items := make([]domain.SaleItem, 0, len(in.Items))
	for _, item := range in.Items {
		_, retailPrice, err := uc.inventory.DeductStock(ctx, item.ProductID, item.Quantity, "")
		if err != nil {
			return nil, err
		}
		items = append(items, domain.SaleItem{
			ProductID:    item.ProductID,
			Quantity:     item.Quantity,
			PricePerUnit: retailPrice,
		})
	}

	sale, err := domain.NewSale(items)
	if err != nil {
		return nil, err
	}

	if err := uc.sales.Create(ctx, sale); err != nil {
		return nil, err
	}

	return sale, nil
}

func (uc *SalesUseCase) GetSale(ctx context.Context, id string) (*domain.Sale, error) {
	return uc.sales.GetByID(ctx, id)
}

func (uc *SalesUseCase) ListSales(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return uc.sales.List(ctx, page, pageSize)
}
