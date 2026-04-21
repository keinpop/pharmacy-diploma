package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockProductRepo struct{ mock.Mock }

func (m *mockProductRepo) Create(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockProductRepo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	args := m.Called(ctx, id)
	if p, ok := args.Get(0).(*domain.Product); ok {
		return p, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockProductRepo) List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]*domain.Product), args.Int(1), args.Error(2)
}
func (m *mockProductRepo) Update(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}

type mockBatchRepo struct{ mock.Mock }

func (m *mockBatchRepo) Create(ctx context.Context, b *domain.Batch) error {
	return m.Called(ctx, b).Error(0)
}
func (m *mockBatchRepo) GetByID(ctx context.Context, id string) (*domain.Batch, error) {
	args := m.Called(ctx, id)
	if b, ok := args.Get(0).(*domain.Batch); ok {
		return b, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockBatchRepo) ListByProduct(ctx context.Context, productID string) ([]*domain.Batch, error) {
	args := m.Called(ctx, productID)
	return args.Get(0).([]*domain.Batch), args.Error(1)
}
func (m *mockBatchRepo) ListExpiring(ctx context.Context, daysAhead int) ([]*domain.Batch, error) {
	args := m.Called(ctx, daysAhead)
	return args.Get(0).([]*domain.Batch), args.Error(1)
}
func (m *mockBatchRepo) ListExpired(ctx context.Context) ([]*domain.Batch, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Batch), args.Error(1)
}
func (m *mockBatchRepo) UpdateStatus(ctx context.Context, id string, status domain.BatchStatus) error {
	return m.Called(ctx, id, status).Error(0)
}
func (m *mockBatchRepo) Save(ctx context.Context, b *domain.Batch) error {
	return m.Called(ctx, b).Error(0)
}

type mockStockRepo struct{ mock.Mock }

func (m *mockStockRepo) GetByProduct(ctx context.Context, productID string) (*domain.StockItem, error) {
	args := m.Called(ctx, productID)
	if s, ok := args.Get(0).(*domain.StockItem); ok {
		return s, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockStockRepo) Upsert(ctx context.Context, s *domain.StockItem) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockStockRepo) ListLowStock(ctx context.Context) ([]*domain.StockItem, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.StockItem), args.Error(1)
}

type mockSearchRepo struct{ mock.Mock }

func (m *mockSearchRepo) IndexProduct(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}
func (m *mockSearchRepo) Search(ctx context.Context, query string, limit int) ([]*domain.Product, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).([]*domain.Product), args.Error(1)
}
func (m *mockSearchRepo) SearchBySubstance(ctx context.Context, substance string, limit int) ([]*domain.Product, error) {
	args := m.Called(ctx, substance, limit)
	return args.Get(0).([]*domain.Product), args.Error(1)
}

func (m *mockSearchRepo) ReindexAll(ctx context.Context, products []*domain.Product) error {
	args := m.Called(ctx, products)
	return args.Error(0)
}

// newUC собирает use-case из моков.
func newUC(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo, es *mockSearchRepo) *usecase.InventoryUseCase {
	return usecase.NewInventoryUseCase(br, sr, pr, es, nil, 30)
}

// stubProduct — хелпер для часто используемого продукта в мок-ответах.
func stubProduct(id string) *domain.Product {
	return &domain.Product{ID: id, Name: "Test", ActiveSubstance: "sub", ReorderPoint: 10}
}

// ── CreateProduct ─────────────────────────────────────────────────────────────

func TestInventoryUseCase_CreateProduct(t *testing.T) {
	tests := []struct {
		name    string
		input   usecase.CreateProductInput
		setup   func(*mockProductRepo, *mockSearchRepo)
		wantErr error
	}{
		{
			name: "success",
			input: usecase.CreateProductInput{
				Name: "Аспирин", ActiveSubstance: "аск",
				Category: domain.CategoryOTC, Unit: "таблетка", ReorderPoint: 10,
			},
			setup: func(pr *mockProductRepo, es *mockSearchRepo) {
				pr.On("Create", mock.Anything, mock.AnythingOfType("*domain.Product")).Return(nil)
				es.On("IndexProduct", mock.Anything, mock.AnythingOfType("*domain.Product")).Return(nil)
			},
		},
		{
			name:    "empty name",
			input:   usecase.CreateProductInput{ActiveSubstance: "аск", Category: domain.CategoryOTC},
			setup:   func(pr *mockProductRepo, es *mockSearchRepo) {},
			wantErr: domain.ErrEmptyProductName,
		},
		{
			name:    "empty active substance",
			input:   usecase.CreateProductInput{Name: "Аспирин", Category: domain.CategoryOTC},
			setup:   func(pr *mockProductRepo, es *mockSearchRepo) {},
			wantErr: domain.ErrEmptyActiveSubstance,
		},
		{
			name:    "invalid category",
			input:   usecase.CreateProductInput{Name: "Аспирин", ActiveSubstance: "аск", Category: "unknown"},
			setup:   func(pr *mockProductRepo, es *mockSearchRepo) {},
			wantErr: domain.ErrInvalidCategory,
		},
		{
			name: "db error on create",
			input: usecase.CreateProductInput{
				Name: "Аспирин", ActiveSubstance: "аск", Category: domain.CategoryOTC,
			},
			setup: func(pr *mockProductRepo, es *mockSearchRepo) {
				pr.On("Create", mock.Anything, mock.AnythingOfType("*domain.Product")).Return(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, es)
			uc := newUC(pr, br, sr, es)

			product, err := uc.CreateProduct(context.Background(), tc.input)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, product)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, product)
				assert.NotEmpty(t, product.ID)
			}
			pr.AssertExpectations(t)
			es.AssertExpectations(t)
		})
	}
}

// ── GetProduct ────────────────────────────────────────────────────────────────

func TestInventoryUseCase_GetProduct(t *testing.T) {
	existing := &domain.Product{ID: "p1", Name: "Аспирин"}

	tests := []struct {
		name    string
		id      string
		setup   func(*mockProductRepo)
		wantErr error
	}{
		{
			name: "found",
			id:   "p1",
			setup: func(pr *mockProductRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(existing, nil)
			},
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(pr *mockProductRepo) {
				pr.On("GetByID", mock.Anything, "bad").Return(nil, usecase.ErrProductNotFound)
			},
			wantErr: usecase.ErrProductNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr)
			uc := newUC(pr, br, sr, es)

			product, err := uc.GetProduct(context.Background(), tc.id)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, product)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, existing.ID, product.ID)
			}
			pr.AssertExpectations(t)
		})
	}
}

// ── SearchProducts ────────────────────────────────────────────────────────────

func TestInventoryUseCase_SearchProducts(t *testing.T) {
	results := []*domain.Product{
		{ID: "p1", Name: "Аспирин"},
		{ID: "p2", Name: "Аспирин Кардио"},
	}

	tests := []struct {
		name      string
		query     string
		limit     int
		setup     func(*mockSearchRepo)
		wantCount int
		wantErr   error
	}{
		{
			name:  "results found",
			query: "аспирин", limit: 10,
			setup: func(es *mockSearchRepo) {
				es.On("Search", mock.Anything, "аспирин", 10).Return(results, nil)
			},
			wantCount: 2,
		},
		{
			name:  "empty query",
			query: "", limit: 10,
			setup:   func(es *mockSearchRepo) {},
			wantErr: domain.ErrEmptySearchQuery,
		},
		{
			name:  "search repo error",
			query: "аспирин", limit: 5,
			setup: func(es *mockSearchRepo) {
				es.On("Search", mock.Anything, "аспирин", 5).Return([]*domain.Product{}, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(es)
			uc := newUC(pr, br, sr, es)

			products, err := uc.SearchProducts(context.Background(), tc.query, tc.limit)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, products, tc.wantCount)
			}
			es.AssertExpectations(t)
		})
	}
}

// ── GetAnalogs ────────────────────────────────────────────────────────────────

func TestInventoryUseCase_GetAnalogs(t *testing.T) {
	original := &domain.Product{ID: "p1", Name: "Аспирин", ActiveSubstance: "аск"}
	analogs := []*domain.Product{{ID: "p2", Name: "Тромбо АСС", ActiveSubstance: "аск"}}

	tests := []struct {
		name      string
		productID string
		limit     int
		setup     func(*mockProductRepo, *mockSearchRepo)
		wantCount int
		wantErr   error
	}{
		{
			name:      "analogs found",
			productID: "p1", limit: 5,
			setup: func(pr *mockProductRepo, es *mockSearchRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(original, nil)
				es.On("SearchBySubstance", mock.Anything, "аск", 6).Return(analogs, nil)
			},
			wantCount: 1,
		},
		{
			name:      "product not found",
			productID: "bad", limit: 5,
			setup: func(pr *mockProductRepo, es *mockSearchRepo) {
				pr.On("GetByID", mock.Anything, "bad").Return(nil, usecase.ErrProductNotFound)
			},
			wantErr: usecase.ErrProductNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, es)
			uc := newUC(pr, br, sr, es)

			products, err := uc.GetAnalogs(context.Background(), tc.productID, tc.limit)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, products, tc.wantCount)
			}
			pr.AssertExpectations(t)
			es.AssertExpectations(t)
		})
	}
}

// ── ReceiveBatch ──────────────────────────────────────────────────────────────

func TestInventoryUseCase_ReceiveBatch(t *testing.T) {
	// validInput используется в нескольких кейсах
	validInput := usecase.ReceiveBatchInput{
		ProductID:     "p1",
		SeriesNumber:  "SER-001",
		ExpiresAt:     time.Now().AddDate(0, 6, 0),
		Quantity:      100,
		PurchasePrice: 50.0,
		RetailPrice:   80.0,
	}

	tests := []struct {
		name    string
		input   usecase.ReceiveBatchInput
		setup   func(*mockProductRepo, *mockBatchRepo, *mockStockRepo)
		wantErr error
	}{
		{
			name:  "success",
			input: validInput,
			// recalcStock: ListByProduct → GetByID → Upsert
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				br.On("Create", mock.Anything, mock.AnythingOfType("*domain.Batch")).Return(nil)
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{}, nil)
				sr.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.StockItem")).Return(nil)
			},
		},
		{
			// domain.NewBatch вызывается ДО GetByID — DB не трогается
			name:    "zero quantity",
			input:   usecase.ReceiveBatchInput{ProductID: "p1", SeriesNumber: "S1", ExpiresAt: time.Now().AddDate(0, 6, 0), Quantity: 0},
			setup:   func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {},
			wantErr: usecase.ErrInvalidQuantity,
		},
		{
			name:    "expired batch",
			input:   usecase.ReceiveBatchInput{ProductID: "p1", SeriesNumber: "S1", ExpiresAt: time.Now().AddDate(0, 0, -1), Quantity: 10},
			setup:   func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {},
			wantErr: usecase.ErrBatchAlreadyExpired,
		},
		{
			name:  "product not found",
			input: validInput,
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(nil, usecase.ErrProductNotFound)
			},
			wantErr: usecase.ErrProductNotFound,
		},
		{
			name:  "batch create db error",
			input: validInput,
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				br.On("Create", mock.Anything, mock.AnythingOfType("*domain.Batch")).Return(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, br, sr)
			uc := newUC(pr, br, sr, es)

			batch, err := uc.ReceiveBatch(context.Background(), tc.input)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, batch)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, batch)
				assert.Equal(t, tc.input.ProductID, batch.ProductID)
			}
			pr.AssertExpectations(t)
			br.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── GetStock ──────────────────────────────────────────────────────────────────

func TestInventoryUseCase_GetStock(t *testing.T) {
	stock := &domain.StockItem{ProductID: "p1", TotalQuantity: 100, Reserved: 10, ReorderPoint: 15}

	tests := []struct {
		name      string
		productID string
		setup     func(*mockProductRepo, *mockStockRepo)
		wantErr   error
	}{
		{
			name:      "found",
			productID: "p1",
			setup: func(pr *mockProductRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				sr.On("GetByProduct", mock.Anything, "p1").Return(stock, nil)
			},
		},
		{
			name:      "product not found",
			productID: "p1",
			setup: func(pr *mockProductRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(nil, usecase.ErrProductNotFound)
			},
			wantErr: usecase.ErrProductNotFound,
		},
		{
			name:      "stock repo error",
			productID: "p1",
			setup: func(pr *mockProductRepo, sr *mockStockRepo) {
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				sr.On("GetByProduct", mock.Anything, "p1").Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, sr)
			uc := newUC(pr, br, sr, es)

			result, err := uc.GetStock(context.Background(), tc.productID)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, stock.ProductID, result.ProductID)
				assert.Equal(t, 90, result.Available())
			}
			pr.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── DeductStock ───────────────────────────────────────────────────────────────

func TestInventoryUseCase_DeductStock(t *testing.T) {
	availableBatch := &domain.Batch{
		ID:        "b1",
		ProductID: "p1",
		ExpiresAt: time.Now().AddDate(0, 6, 0),
		Quantity:  100, Reserved: 0,
		Status: domain.BatchStatusAvailable,
	}

	tests := []struct {
		name      string
		productID string
		qty       int
		orderID   string
		setup     func(*mockProductRepo, *mockBatchRepo, *mockStockRepo)
		wantErr   error
	}{
		{
			name:      "success",
			productID: "p1", qty: 10, orderID: "ord-1",
			// recalcStock: ListByProduct (повторный) → GetByID → Upsert
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{availableBatch}, nil)
				br.On("Save", mock.Anything, mock.AnythingOfType("*domain.Batch")).Return(nil)
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				sr.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.StockItem")).Return(nil)
			},
		},
		{
			name:      "insufficient stock",
			productID: "p1", qty: 200, orderID: "ord-2",
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{availableBatch}, nil)
			},
			wantErr: domain.ErrInsufficientStock,
		},
		{
			name:      "no batches",
			productID: "p1", qty: 5, orderID: "ord-3",
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{}, nil)
			},
			wantErr: domain.ErrInsufficientStock,
		},
		{
			name:      "list batches db error",
			productID: "p1", qty: 5, orderID: "ord-4",
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{}, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, br, sr)
			uc := newUC(pr, br, sr, es)

			batchID, _, _, _, err := uc.DeductStock(context.Background(), tc.productID, tc.qty, tc.orderID)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Empty(t, batchID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, availableBatch.ID, batchID)
			}
			pr.AssertExpectations(t)
			br.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}

// ── ListExpiringBatches ───────────────────────────────────────────────────────

func TestInventoryUseCase_ListExpiringBatches(t *testing.T) {
	expiring := []*domain.Batch{
		{ID: "b1", ProductID: "p1", ExpiresAt: time.Now().AddDate(0, 0, 10), Status: domain.BatchStatusExpiringSoon},
	}

	tests := []struct {
		name      string
		daysAhead int
		setup     func(*mockBatchRepo)
		wantCount int
		wantErr   error
	}{
		{
			name:      "has expiring batches",
			daysAhead: 30,
			setup: func(br *mockBatchRepo) {
				br.On("ListExpiring", mock.Anything, 30).Return(expiring, nil)
			},
			wantCount: 1,
		},
		{
			name:      "no expiring batches",
			daysAhead: 7,
			setup: func(br *mockBatchRepo) {
				br.On("ListExpiring", mock.Anything, 7).Return([]*domain.Batch{}, nil)
			},
			wantCount: 0,
		},
		{
			name:      "repo error",
			daysAhead: 30,
			setup: func(br *mockBatchRepo) {
				br.On("ListExpiring", mock.Anything, 30).Return([]*domain.Batch{}, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(br)
			uc := newUC(pr, br, sr, es)

			result, err := uc.ListExpiringBatches(context.Background(), tc.daysAhead)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
			}
			br.AssertExpectations(t)
		})
	}
}

// ── ListLowStock ──────────────────────────────────────────────────────────────

func TestInventoryUseCase_ListLowStock(t *testing.T) {
	lowItems := []*domain.StockItem{
		{ProductID: "p1", TotalQuantity: 5, Reserved: 0, ReorderPoint: 10},
		{ProductID: "p2", TotalQuantity: 3, Reserved: 1, ReorderPoint: 10},
	}

	tests := []struct {
		name      string
		setup     func(*mockStockRepo)
		wantCount int
		wantErr   error
	}{
		{
			name: "has low stock items",
			setup: func(sr *mockStockRepo) {
				sr.On("ListLowStock", mock.Anything).Return(lowItems, nil)
			},
			wantCount: 2,
		},
		{
			name: "empty",
			setup: func(sr *mockStockRepo) {
				sr.On("ListLowStock", mock.Anything).Return([]*domain.StockItem{}, nil)
			},
			wantCount: 0,
		},
		{
			name: "repo error",
			setup: func(sr *mockStockRepo) {
				sr.On("ListLowStock", mock.Anything).Return([]*domain.StockItem{}, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(sr)
			uc := newUC(pr, br, sr, es)

			result, err := uc.ListLowStock(context.Background())

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
			}
			sr.AssertExpectations(t)
		})
	}
}

// ── WriteOffExpired ───────────────────────────────────────────────────────────

func TestInventoryUseCase_WriteOffExpired(t *testing.T) {
	now := time.Now()
	expiredBatches := []*domain.Batch{
		{ID: "b1", ProductID: "p1", ExpiresAt: now.AddDate(0, 0, -3), Quantity: 20, Status: domain.BatchStatusExpired},
		{ID: "b2", ProductID: "p2", ExpiresAt: now.AddDate(0, 0, -1), Quantity: 5, Status: domain.BatchStatusExpired},
	}

	tests := []struct {
		name           string
		setup          func(*mockProductRepo, *mockBatchRepo, *mockStockRepo)
		wantWrittenOff int
		wantErr        error
	}{
		{
			name: "two batches written off",
			// recalcStock вызывается дважды — для p1 и p2
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListExpired", mock.Anything).Return(expiredBatches, nil)
				br.On("UpdateStatus", mock.Anything, "b1", domain.BatchStatusWrittenOff).Return(nil)
				br.On("UpdateStatus", mock.Anything, "b2", domain.BatchStatusWrittenOff).Return(nil)
				br.On("ListByProduct", mock.Anything, "p1").Return([]*domain.Batch{}, nil)
				br.On("ListByProduct", mock.Anything, "p2").Return([]*domain.Batch{}, nil)
				pr.On("GetByID", mock.Anything, "p1").Return(stubProduct("p1"), nil)
				pr.On("GetByID", mock.Anything, "p2").Return(stubProduct("p2"), nil)
				sr.On("Upsert", mock.Anything, mock.AnythingOfType("*domain.StockItem")).Return(nil)
			},
			wantWrittenOff: 2,
		},
		{
			name: "no expired batches",
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListExpired", mock.Anything).Return([]*domain.Batch{}, nil)
			},
			wantWrittenOff: 0,
		},
		{
			name: "list expired db error",
			setup: func(pr *mockProductRepo, br *mockBatchRepo, sr *mockStockRepo) {
				br.On("ListExpired", mock.Anything).Return([]*domain.Batch{}, assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pr, br, sr, es := &mockProductRepo{}, &mockBatchRepo{}, &mockStockRepo{}, &mockSearchRepo{}
			tc.setup(pr, br, sr)
			uc := newUC(pr, br, sr, es)

			count, err := uc.WriteOffExpired(context.Background())

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantWrittenOff, count)
			}
			pr.AssertExpectations(t)
			br.AssertExpectations(t)
			sr.AssertExpectations(t)
		})
	}
}
