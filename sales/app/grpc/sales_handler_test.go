package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcapp "pharmacy/sales/app/grpc"
	"pharmacy/sales/domain"
	usecase "pharmacy/sales/domain/use_case"
	pb "pharmacy/sales/gen/sales"
)

type MockUC struct{ mock.Mock }

func (m *MockUC) CreateSale(ctx context.Context, in usecase.CreateSaleInput) (*domain.Sale, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*domain.Sale); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) GetSale(ctx context.Context, id string) (*domain.Sale, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Sale); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) ListSales(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]*domain.Sale), args.Int(1), args.Error(2)
}

func TestHandler_CreateSale(t *testing.T) {
	t.Parallel()
	saleStub := &domain.Sale{
		ID: "sale-1", TotalAmount: 300, SoldAt: time.Now(),
		SellerUsername: "alice",
		Items: []domain.SaleItem{
			{ID: "item-1", SaleID: "sale-1", ProductID: "p1", Quantity: 2, PricePerUnit: 150, TotalPrice: 300},
		},
	}

	tests := []struct {
		name      string
		req       *pb.CreateSaleRequest
		setup     func(*MockUC)
		wantCode  codes.Code
		wantSale  string
		wantItems int
	}{
		{
			name: "успех",
			req: &pb.CreateSaleRequest{
				Items: []*pb.SaleItemRequest{{ProductId: "p1", Quantity: 2}},
			},
			setup: func(m *MockUC) {
				m.On("CreateSale", mock.Anything, mock.MatchedBy(func(in usecase.CreateSaleInput) bool {
					return len(in.Items) == 1 && in.Items[0].ProductID == "p1"
				})).Return(saleStub, nil)
			},
			wantSale:  "sale-1",
			wantItems: 1,
		},
		{
			name:     "пустой список — InvalidArgument",
			req:      &pb.CreateSaleRequest{Items: nil},
			setup:    func(m *MockUC) {},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "use case вернул ErrEmptySeller — InvalidArgument",
			req: &pb.CreateSaleRequest{
				Items: []*pb.SaleItemRequest{{ProductId: "p1", Quantity: 2}},
			},
			setup: func(m *MockUC) {
				m.On("CreateSale", mock.Anything, mock.AnythingOfType("usecase.CreateSaleInput")).
					Return(nil, domain.ErrEmptySeller)
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "use case вернул внутреннюю ошибку — Internal",
			req: &pb.CreateSaleRequest{
				Items: []*pb.SaleItemRequest{{ProductId: "p1", Quantity: 2}},
			},
			setup: func(m *MockUC) {
				m.On("CreateSale", mock.Anything, mock.AnythingOfType("usecase.CreateSaleInput")).
					Return(nil, errors.New("oops"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := new(MockUC)
			tc.setup(uc)
			h := grpcapp.NewHandler(uc)

			resp, err := h.CreateSale(context.Background(), tc.req)

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSale, resp.GetSale().GetId())
			assert.Len(t, resp.GetSale().GetItems(), tc.wantItems)
			assert.Equal(t, "alice", resp.GetSale().GetSellerUsername())
		})
	}
}

func TestHandler_GetSale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		id       string
		setup    func(*MockUC)
		wantCode codes.Code
	}{
		{
			name: "найдено",
			id:   "sale-1",
			setup: func(m *MockUC) {
				m.On("GetSale", mock.Anything, "sale-1").
					Return(&domain.Sale{ID: "sale-1", SoldAt: time.Now()}, nil)
			},
		},
		{
			name: "не найдено",
			id:   "bad",
			setup: func(m *MockUC) {
				m.On("GetSale", mock.Anything, "bad").Return(nil, domain.ErrSaleNotFound)
			},
			wantCode: codes.NotFound,
		},
		{
			name: "внутренняя ошибка",
			id:   "id",
			setup: func(m *MockUC) {
				m.On("GetSale", mock.Anything, "id").Return(nil, errors.New("db down"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := new(MockUC)
			tc.setup(uc)
			h := grpcapp.NewHandler(uc)

			resp, err := h.GetSale(context.Background(), &pb.GetSaleRequest{Id: tc.id})

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, resp.GetSale().GetId())
		})
	}
}

func TestHandler_ListSales(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		req      *pb.ListSalesRequest
		setup    func(*MockUC)
		wantLen  int
		wantCode codes.Code
	}{
		{
			name: "одна продажа",
			req:  &pb.ListSalesRequest{Page: 1, PageSize: 10},
			setup: func(m *MockUC) {
				m.On("ListSales", mock.Anything, 1, 10).
					Return([]*domain.Sale{{ID: "s1", SoldAt: time.Now()}}, 1, nil)
			},
			wantLen: 1,
		},
		{
			name: "ошибка БД",
			req:  &pb.ListSalesRequest{Page: 1, PageSize: 10},
			setup: func(m *MockUC) {
				m.On("ListSales", mock.Anything, 1, 10).
					Return([]*domain.Sale{}, 0, errors.New("db"))
			},
			wantCode: codes.Internal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := new(MockUC)
			tc.setup(uc)
			h := grpcapp.NewHandler(uc)

			resp, err := h.ListSales(context.Background(), tc.req)

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.GetSales(), tc.wantLen)
		})
	}
}
