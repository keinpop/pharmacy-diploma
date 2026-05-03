package usecase

import (
	"context"

	"pharmacy/sales/app/metrics"
	"pharmacy/sales/domain"
)

// SellerProvider возвращает имя пользователя, оформившего вызов.
// Реализуется в transport-слое (gRPC interceptor сохраняет username
// в context, see app/grpc/interceptor.go).
type SellerProvider func(ctx context.Context) string

type SalesUseCase struct {
	sales         SaleRepository
	inventory     InventoryClient
	events        EventPublisher
	resolveSeller SellerProvider
}

func NewSalesUseCase(sales SaleRepository, inventory InventoryClient, events EventPublisher, resolveSeller SellerProvider) *SalesUseCase {
	if resolveSeller == nil {
		resolveSeller = func(context.Context) string { return "" }
	}
	return &SalesUseCase{
		sales:         sales,
		inventory:     inventory,
		events:        events,
		resolveSeller: resolveSeller,
	}
}

func (uc *SalesUseCase) CreateSale(ctx context.Context, in CreateSaleInput) (*domain.Sale, error) {
	if len(in.Items) == 0 {
		return nil, domain.ErrEmptyItems
	}

	seller := uc.resolveSeller(ctx)
	if seller == "" {
		return nil, domain.ErrEmptySeller
	}

	type itemInfo struct {
		productName      string
		therapeuticGroup string
	}
	infos := make([]itemInfo, 0, len(in.Items))
	items := make([]domain.SaleItem, 0, len(in.Items))

	for _, item := range in.Items {
		_, retailPrice, productName, therapeuticGroup, err := uc.inventory.DeductStock(ctx, item.ProductID, item.Quantity, "")
		if err != nil {
			return nil, err
		}
		items = append(items, domain.SaleItem{
			ProductID:    item.ProductID,
			Quantity:     item.Quantity,
			PricePerUnit: retailPrice,
		})
		infos = append(infos, itemInfo{productName: productName, therapeuticGroup: therapeuticGroup})
	}

	sale, err := domain.NewSale(items, seller)
	if err != nil {
		return nil, err
	}

	if err := uc.sales.Create(ctx, sale); err != nil {
		return nil, err
	}

	metrics.SalesCreated.Inc()
	metrics.SaleAmount.Observe(sale.TotalAmount)

	if uc.events != nil {
		for i, item := range sale.Items {
			_ = uc.events.PublishSaleCompleted(ctx, SaleCompletedEvent{
				ProductID:        item.ProductID,
				ProductName:      infos[i].productName,
				TherapeuticGroup: infos[i].therapeuticGroup,
				Quantity:         item.Quantity,
				PricePerUnit:     item.PricePerUnit,
				TotalPrice:       float64(item.Quantity) * item.PricePerUnit,
				SoldAt:           sale.SoldAt,
				SellerUsername:   sale.SellerUsername,
			})
		}
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
