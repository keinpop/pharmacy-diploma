package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pharmacy/sales/domain"
	usecase "pharmacy/sales/domain/use_case"
)

// — mocks —

type MockSaleRepo struct{ mock.Mock }

func (m *MockSaleRepo) Create(ctx context.Context, sale *domain.Sale) error {
	return m.Called(ctx, sale).Error(0)
}
func (m *MockSaleRepo) GetByID(ctx context.Context, id string) (*domain.Sale, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Sale); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockSaleRepo) List(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]*domain.Sale), args.Int(1), args.Error(2)
}

type MockInventoryClient struct{ mock.Mock }

func (m *MockInventoryClient) DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, string, string, error) {
	args := m.Called(ctx, productID, qty, orderID)
	return args.String(0), args.Get(1).(float64), args.String(2), args.String(3), args.Error(4)
}

type MockEventPublisher struct{ mock.Mock }

func (m *MockEventPublisher) PublishSaleCompleted(ctx context.Context, event usecase.SaleCompletedEvent) error {
	return m.Called(ctx, event).Error(0)
}

// staticSeller возвращает фиксированный username из контекста.
func staticSeller(name string) usecase.SellerProvider {
	return func(context.Context) string { return name }
}

// — tests —

func TestSalesUseCase_CreateSale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       usecase.CreateSaleInput
		seller      usecase.SellerProvider
		setupMocks  func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher)
		wantErr     error
		wantItemQty int
		wantSeller  string
	}{
		{
			name:       "пустой список позиций",
			input:      usecase.CreateSaleInput{Items: nil},
			seller:     staticSeller("alice"),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {},
			wantErr:    domain.ErrEmptyItems,
		},
		{
			name: "отсутствует seller в context",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{{ProductID: "p1", Quantity: 1}},
			},
			seller:     staticSeller(""),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {},
			wantErr:    domain.ErrEmptySeller,
		},
		{
			name: "ошибка списания со склада",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{{ProductID: "p1", Quantity: 5}},
			},
			seller: staticSeller("alice"),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {
				inv.On("DeductStock", mock.Anything, "p1", 5, "").
					Return("", float64(0), "", "", errors.New("insufficient stock"))
			},
			wantErr: errors.New("insufficient stock"),
		},
		{
			name: "успешная одиночная позиция",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{{ProductID: "p1", Quantity: 3}},
			},
			seller: staticSeller("alice"),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {
				inv.On("DeductStock", mock.Anything, "p1", 3, "").
					Return("batch-1", float64(150.50), "Aspirin", "анальгетики", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).Return(nil)
				pub.On("PublishSaleCompleted", mock.Anything, mock.AnythingOfType("usecase.SaleCompletedEvent")).Return(nil)
			},
			wantItemQty: 1,
			wantSeller:  "alice",
		},
		{
			name: "успешные несколько позиций",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{
					{ProductID: "p1", Quantity: 2},
					{ProductID: "p2", Quantity: 1},
				},
			},
			seller: staticSeller("bob"),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {
				inv.On("DeductStock", mock.Anything, "p1", 2, "").
					Return("batch-1", float64(100), "Aspirin", "анальгетики", nil)
				inv.On("DeductStock", mock.Anything, "p2", 1, "").
					Return("batch-2", float64(200), "Loratadine", "антигистаминные", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).Return(nil)
				pub.On("PublishSaleCompleted", mock.Anything, mock.AnythingOfType("usecase.SaleCompletedEvent")).Return(nil)
			},
			wantItemQty: 2,
			wantSeller:  "bob",
		},
		{
			name: "ошибка БД при сохранении",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{{ProductID: "p1", Quantity: 1}},
			},
			seller: staticSeller("alice"),
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient, pub *MockEventPublisher) {
				inv.On("DeductStock", mock.Anything, "p1", 1, "").
					Return("batch-1", float64(50), "Aspirin", "анальгетики", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).
					Return(errors.New("db error"))
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			pub := new(MockEventPublisher)
			tc.setupMocks(repo, inv, pub)

			uc := usecase.NewSalesUseCase(repo, inv, pub, tc.seller)
			sale, err := uc.CreateSale(context.Background(), tc.input)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr.Error())
				assert.Nil(t, sale)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sale)
			assert.NotEmpty(t, sale.ID)
			assert.Equal(t, tc.wantItemQty, len(sale.Items))
			assert.Equal(t, tc.wantSeller, sale.SellerUsername)
		})
	}
}

func TestSalesUseCase_GetSale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		id      string
		setup   func(repo *MockSaleRepo)
		wantErr error
	}{
		{
			name: "найдено",
			id:   "sale-1",
			setup: func(repo *MockSaleRepo) {
				repo.On("GetByID", mock.Anything, "sale-1").
					Return(&domain.Sale{ID: "sale-1", SellerUsername: "alice"}, nil)
			},
		},
		{
			name: "не найдено",
			id:   "bad",
			setup: func(repo *MockSaleRepo) {
				repo.On("GetByID", mock.Anything, "bad").Return(nil, domain.ErrSaleNotFound)
			},
			wantErr: domain.ErrSaleNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			tc.setup(repo)

			uc := usecase.NewSalesUseCase(repo, inv, nil, staticSeller("alice"))
			sale, err := uc.GetSale(context.Background(), tc.id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, sale.ID)
		})
	}
}

func TestSalesUseCase_ListSales(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		page     int
		pageSize int
		setup    func(repo *MockSaleRepo)
		wantLen  int
		wantErr  bool
	}{
		{
			name: "page=0, pageSize=0 — применяются дефолты 1/20",
			page: 0, pageSize: 0,
			setup: func(repo *MockSaleRepo) {
				repo.On("List", mock.Anything, 1, 20).
					Return([]*domain.Sale{{ID: "s1"}}, 1, nil)
			},
			wantLen: 1,
		},
		{
			name: "кастомная пагинация",
			page: 2, pageSize: 5,
			setup: func(repo *MockSaleRepo) {
				repo.On("List", mock.Anything, 2, 5).
					Return([]*domain.Sale{}, 0, nil)
			},
			wantLen: 0,
		},
		{
			name: "ошибка БД",
			page: 1, pageSize: 10,
			setup: func(repo *MockSaleRepo) {
				repo.On("List", mock.Anything, 1, 10).
					Return([]*domain.Sale{}, 0, errors.New("db"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			tc.setup(repo)

			uc := usecase.NewSalesUseCase(repo, inv, nil, staticSeller("alice"))
			sales, _, err := uc.ListSales(context.Background(), tc.page, tc.pageSize)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, sales, tc.wantLen)
		})
	}
}
