package usecase

import (
	"context"
	"fmt"
	"sort"

	"pharmacy/inventory/domain"
)

type InventoryUseCase struct {
	batches          BatchRepository
	stock            StockRepository
	products         ProductRepository
	search           SearchRepository
	expiringSoonDays int
}

func NewInventoryUseCase(
	b BatchRepository,
	s StockRepository,
	p ProductRepository,
	sr SearchRepository,
	expiringSoonDays int,
) *InventoryUseCase {
	return &InventoryUseCase{batches: b, stock: s, products: p, search: sr, expiringSoonDays: expiringSoonDays}
}

// Product

func (uc *InventoryUseCase) CreateProduct(ctx context.Context, in CreateProductInput) (*domain.Product, error) {
	p, err := domain.NewProduct(
		in.Name, in.TradeName, in.ActiveSubstance,
		in.Form, in.Dosage, in.Category,
		in.StorageConditions, in.Unit, in.ReorderPoint,
	)
	if err != nil {
		return nil, err
	}
	if err := uc.products.Create(ctx, p); err != nil {
		return nil, err
	}
	err = uc.search.IndexProduct(ctx, p)
	if err != nil {
		fmt.Println("CreateProduct can't index")
	}
	return p, nil
}

func (uc *InventoryUseCase) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	return uc.products.GetByID(ctx, id)
}

func (uc *InventoryUseCase) ListProducts(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return uc.products.List(ctx, page, pageSize)
}

func (uc *InventoryUseCase) SearchProducts(ctx context.Context, query string, limit int) ([]*domain.Product, error) {
	if query == "" {
		return nil, domain.ErrEmptySearchQuery
	}
	if limit <= 0 {
		limit = 20
	}
	return uc.search.Search(ctx, query, limit)
}

func (uc *InventoryUseCase) GetAnalogs(ctx context.Context, productID string, limit int) ([]*domain.Product, error) {
	if limit <= 0 {
		limit = 5
	}
	p, err := uc.products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	results, err := uc.search.SearchBySubstance(ctx, p.ActiveSubstance, limit+1)
	if err != nil {
		return nil, err
	}
	analogs := make([]*domain.Product, 0, limit)
	for _, r := range results {
		if r.ID != productID {
			analogs = append(analogs, r)
		}
		if len(analogs) == limit {
			break
		}
	}
	return analogs, nil
}

// Inventory

// ReceiveBatch - получение партии - сначала валидируем доменные правила, потом идём в БД.
func (uc *InventoryUseCase) ReceiveBatch(ctx context.Context, in ReceiveBatchInput) (*domain.Batch, error) {
	// валидация без IO (нулевое количество, уже истёкший срок)
	b, err := domain.NewBatch(
		in.ProductID, in.SeriesNumber, in.ExpiresAt,
		in.Quantity, in.PurchasePrice, in.RetailPrice,
		uc.expiringSoonDays,
	)
	if err != nil {
		return nil, err
	}
	// проверка что продукт существует
	if _, err := uc.products.GetByID(ctx, in.ProductID); err != nil {
		return nil, err
	}
	if err := uc.batches.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, uc.recalcStock(ctx, in.ProductID)
}

func (uc *InventoryUseCase) GetStock(ctx context.Context, productID string) (*domain.StockItem, error) {
	p, err := uc.products.GetByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	s, err := uc.stock.GetByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	s.ReorderPoint = p.ReorderPoint
	return s, nil
}

// DeductStock списывает qty единиц товара по FEFO.
// Возвращает ID первой использованной партии, средневзвешенную розничную цену и ошибку.
func (uc *InventoryUseCase) DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, error) {
	batches, err := uc.batches.ListByProduct(ctx, productID)
	if err != nil {
		return "", 0, err
	}

	var available []*domain.Batch
	for _, b := range batches {
		if (b.Status == domain.BatchStatusAvailable || b.Status == domain.BatchStatusExpiringSoon) && b.Available() > 0 {
			available = append(available, b)
		}
	}
	sort.Slice(available, func(i, j int) bool {
		return available[i].ExpiresAt.Before(available[j].ExpiresAt)
	})

	remaining := qty
	usedBatchID := ""
	var totalCost float64
	var totalDeducted int

	for _, b := range available {
		if remaining <= 0 {
			break
		}
		deduct := remaining
		if b.Available() < deduct {
			deduct = b.Available()
		}
		if err := b.Deduct(deduct); err != nil {
			return "", 0, err
		}
		if err := uc.batches.Save(ctx, b); err != nil {
			return "", 0, err
		}
		if usedBatchID == "" {
			usedBatchID = b.ID
		}
		totalCost += float64(deduct) * b.RetailPrice
		totalDeducted += deduct
		remaining -= deduct
	}
	if remaining > 0 {
		return "", 0, domain.ErrInsufficientStock
	}

	retailPrice := totalCost / float64(totalDeducted)
	return usedBatchID, retailPrice, uc.recalcStock(ctx, productID)
}

func (uc *InventoryUseCase) ListExpiringBatches(ctx context.Context, daysAhead int) ([]*domain.Batch, error) {
	return uc.batches.ListExpiring(ctx, daysAhead)
}

func (uc *InventoryUseCase) ListLowStock(ctx context.Context) ([]*domain.StockItem, error) {
	return uc.stock.ListLowStock(ctx)
}

// WriteOffExpired джоб для периодического списания. Обходит все просроченные партии, переводит в статус written_off, пересчитывает остатки. Возвращает количество списанных партий.
func (uc *InventoryUseCase) WriteOffExpired(ctx context.Context) (int, error) {
	expired, err := uc.batches.ListExpired(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, b := range expired {
		if err := uc.batches.UpdateStatus(ctx, b.ID, domain.BatchStatusWrittenOff); err != nil {
			return count, err
		}
		count++
		_ = uc.recalcStock(ctx, b.ProductID)
	}
	return count, nil
}

func (uc *InventoryUseCase) recalcStock(ctx context.Context, productID string) error {
	batches, err := uc.batches.ListByProduct(ctx, productID)
	if err != nil {
		return err
	}
	total, reserved := 0, 0
	for _, b := range batches {
		if b.Status != domain.BatchStatusWrittenOff {
			total += b.Quantity
			reserved += b.Reserved
		}
	}
	p, _ := uc.products.GetByID(ctx, productID)
	reorder := 0
	if p != nil {
		reorder = p.ReorderPoint
	}
	return uc.stock.Upsert(ctx, &domain.StockItem{
		ProductID:     productID,
		TotalQuantity: total,
		Reserved:      reserved,
		ReorderPoint:  reorder,
	})
}
