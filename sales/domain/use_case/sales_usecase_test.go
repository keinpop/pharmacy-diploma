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

// — tests —

func TestCreateSale(t *testing.T) {
	tests := []struct {
		name        string
		input       usecase.CreateSaleInput
		setupMocks  func(repo *MockSaleRepo, inv *MockInventoryClient)
		wantErr     bool
		errContains string
	}{
		{
			name:       "empty items",
			input:      usecase.CreateSaleInput{Items: nil},
			wantErr:    true,
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient) {},
		},
		{
			name: "inventory deduct fails",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{
					{ProductID: "p1", Quantity: 5},
				},
			},
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient) {
				inv.On("DeductStock", mock.Anything, "p1", 5, "").
					Return("", float64(0), "", "", errors.New("insufficient stock"))
			},
			wantErr: true,
		},
		{
			name: "single item success",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{
					{ProductID: "p1", Quantity: 3},
				},
			},
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient) {
				inv.On("DeductStock", mock.Anything, "p1", 3, "").
					Return("batch-1", float64(150.50), "Aspirin", "painkiller", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "multiple items success",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{
					{ProductID: "p1", Quantity: 2},
					{ProductID: "p2", Quantity: 1},
				},
			},
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient) {
				inv.On("DeductStock", mock.Anything, "p1", 2, "").
					Return("batch-1", float64(100.0), "Aspirin", "painkiller", nil)
				inv.On("DeductStock", mock.Anything, "p2", 1, "").
					Return("batch-2", float64(200.0), "Loratadine", "antihistamine", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "repo create fails",
			input: usecase.CreateSaleInput{
				Items: []usecase.CreateSaleItemInput{
					{ProductID: "p1", Quantity: 1},
				},
			},
			setupMocks: func(repo *MockSaleRepo, inv *MockInventoryClient) {
				inv.On("DeductStock", mock.Anything, "p1", 1, "").
					Return("batch-1", float64(50.0), "Aspirin", "painkiller", nil)
				repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Sale")).
					Return(errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			tc.setupMocks(repo, inv)

			uc := usecase.NewSalesUseCase(repo, inv, nil)
			sale, err := uc.CreateSale(context.Background(), tc.input)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, sale)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sale)
			assert.NotEmpty(t, sale.ID)
			assert.Equal(t, len(tc.input.Items), len(sale.Items))
		})
	}
}

func TestGetSale(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(repo *MockSaleRepo)
		wantErr bool
	}{
		{
			name: "found",
			id:   "sale-1",
			setup: func(repo *MockSaleRepo) {
				repo.On("GetByID", mock.Anything, "sale-1").
					Return(&domain.Sale{ID: "sale-1"}, nil)
			},
			wantErr: false,
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(repo *MockSaleRepo) {
				repo.On("GetByID", mock.Anything, "bad").
					Return(nil, domain.ErrSaleNotFound)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			tc.setup(repo)

			uc := usecase.NewSalesUseCase(repo, inv, nil)
			sale, err := uc.GetSale(context.Background(), tc.id)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, sale.ID)
		})
	}
}

func TestListSales(t *testing.T) {
	tests := []struct {
		name     string
		page     int
		pageSize int
		setup    func(repo *MockSaleRepo)
		wantLen  int
		wantErr  bool
	}{
		{
			name: "defaults for invalid page",
			page: 0, pageSize: 0,
			setup: func(repo *MockSaleRepo) {
				repo.On("List", mock.Anything, 1, 20).
					Return([]*domain.Sale{{ID: "s1"}}, 1, nil)
			},
			wantLen: 1,
		},
		{
			name: "custom pagination",
			page: 2, pageSize: 5,
			setup: func(repo *MockSaleRepo) {
				repo.On("List", mock.Anything, 2, 5).
					Return([]*domain.Sale{}, 0, nil)
			},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockSaleRepo)
			inv := new(MockInventoryClient)
			tc.setup(repo)

			uc := usecase.NewSalesUseCase(repo, inv, nil)
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
