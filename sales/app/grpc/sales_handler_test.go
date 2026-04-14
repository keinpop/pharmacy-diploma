package grpc_test

import (
	"context"
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

func TestCreateSale_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)

	uc.On("CreateSale", mock.Anything, mock.MatchedBy(func(in usecase.CreateSaleInput) bool {
		return len(in.Items) == 1 && in.Items[0].ProductID == "p1"
	})).Return(&domain.Sale{
		ID:          "sale-1",
		TotalAmount: 300,
		SoldAt:      time.Now(),
		Items: []domain.SaleItem{
			{ID: "item-1", SaleID: "sale-1", ProductID: "p1", Quantity: 2, PricePerUnit: 150, TotalPrice: 300},
		},
	}, nil)

	resp, err := h.CreateSale(context.Background(), &pb.CreateSaleRequest{
		Items: []*pb.SaleItemRequest{{ProductId: "p1", Quantity: 2}},
	})
	require.NoError(t, err)
	assert.Equal(t, "sale-1", resp.Sale.Id)
	assert.Len(t, resp.Sale.Items, 1)
}

func TestCreateSale_EmptyItems(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)

	_, err := h.CreateSale(context.Background(), &pb.CreateSaleRequest{Items: nil})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetSale_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)

	uc.On("GetSale", mock.Anything, "sale-1").
		Return(&domain.Sale{ID: "sale-1", TotalAmount: 500, SoldAt: time.Now()}, nil)

	resp, err := h.GetSale(context.Background(), &pb.GetSaleRequest{Id: "sale-1"})
	require.NoError(t, err)
	assert.Equal(t, "sale-1", resp.Sale.Id)
}

func TestGetSale_NotFound(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)

	uc.On("GetSale", mock.Anything, "bad").Return(nil, domain.ErrSaleNotFound)

	_, err := h.GetSale(context.Background(), &pb.GetSaleRequest{Id: "bad"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestListSales_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)

	uc.On("ListSales", mock.Anything, 1, 10).
		Return([]*domain.Sale{{ID: "s1", SoldAt: time.Now()}}, 1, nil)

	resp, err := h.ListSales(context.Background(), &pb.ListSalesRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Sales, 1)
	assert.Equal(t, int32(1), resp.Total)
}
