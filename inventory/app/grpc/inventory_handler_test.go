package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	grpcapp "pharmacy/inventory/app/grpc"
	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
	pb "pharmacy/inventory/gen/inventory"
)

type MockUC struct{ mock.Mock }

func (m *MockUC) CreateProduct(ctx context.Context, in usecase.CreateProductInput) (*domain.Product, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*domain.Product); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	args := m.Called(ctx, id)
	if v, ok := args.Get(0).(*domain.Product); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) ListProducts(ctx context.Context, page, size int) ([]*domain.Product, int, error) {
	args := m.Called(ctx, page, size)
	return args.Get(0).([]*domain.Product), args.Int(1), args.Error(2)
}
func (m *MockUC) SearchProducts(ctx context.Context, q string, limit int) ([]*domain.Product, error) {
	args := m.Called(ctx, q, limit)
	return args.Get(0).([]*domain.Product), args.Error(1)
}
func (m *MockUC) GetAnalogs(ctx context.Context, id string, limit int) ([]*domain.Product, error) {
	args := m.Called(ctx, id, limit)
	return args.Get(0).([]*domain.Product), args.Error(1)
}
func (m *MockUC) ReceiveBatch(ctx context.Context, in usecase.ReceiveBatchInput) (*domain.Batch, error) {
	args := m.Called(ctx, in)
	if v, ok := args.Get(0).(*domain.Batch); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) GetStock(ctx context.Context, productID string) (*domain.StockItem, error) {
	args := m.Called(ctx, productID)
	if v, ok := args.Get(0).(*domain.StockItem); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *MockUC) DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, error) {
	args := m.Called(ctx, productID, qty, orderID)
	return args.String(0), float64(args.Int(0)), args.Error(1)
}
func (m *MockUC) ListExpiringBatches(ctx context.Context, days int) ([]*domain.Batch, error) {
	args := m.Called(ctx, days)
	return args.Get(0).([]*domain.Batch), args.Error(1)
}
func (m *MockUC) ListLowStock(ctx context.Context) ([]*domain.StockItem, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.StockItem), args.Error(1)
}
func (m *MockUC) WriteOffExpired(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestGetProduct_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)
	ctx := context.Background()

	uc.On("GetProduct", ctx, "p1").Return(&domain.Product{ID: "p1", Name: "Aspirin"}, nil)
	resp, err := h.GetProduct(ctx, &pb.GetProductRequest{Id: "p1"})
	require.NoError(t, err)
	assert.Equal(t, "p1", resp.Product.Id)
}

func TestGetProduct_NotFound(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)
	ctx := context.Background()

	uc.On("GetProduct", ctx, "bad").Return(nil, assert.AnError)
	_, err := h.GetProduct(ctx, &pb.GetProductRequest{Id: "bad"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestDeductStock_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)
	ctx := context.Background()

	uc.On("DeductStock", ctx, "p1", 10, "ord-1").Return("batch-42", nil)
	resp, err := h.DeductStock(ctx, &pb.DeductStockRequest{ProductId: "p1", Quantity: 10, OrderId: "ord-1"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "batch-42", resp.BatchId)
}

func TestWriteOffExpired_OK(t *testing.T) {
	uc := new(MockUC)
	h := grpcapp.NewHandler(uc)
	ctx := context.Background()

	uc.On("WriteOffExpired", ctx).Return(3, nil)
	resp, err := h.WriteOffExpired(ctx, &pb.WriteOffExpiredRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.WrittenOffCount)
}
