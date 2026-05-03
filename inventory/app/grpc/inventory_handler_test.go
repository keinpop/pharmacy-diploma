package grpc_test

import (
	"context"
	"errors"
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

// MockUC mocks the InventoryUC port consumed by gRPC handler.
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
func (m *MockUC) DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, string, string, error) {
	args := m.Called(ctx, productID, qty, orderID)
	return args.String(0), args.Get(1).(float64), args.String(2), args.String(3), args.Error(4)
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

func TestHandler_GetProduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		id       string
		setup    func(*MockUC)
		wantCode codes.Code
		wantID   string
	}{
		{
			name: "success",
			id:   "p1",
			setup: func(m *MockUC) {
				m.On("GetProduct", mock.Anything, "p1").
					Return(&domain.Product{ID: "p1", Name: "Aspirin"}, nil)
			},
			wantID: "p1",
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(m *MockUC) {
				m.On("GetProduct", mock.Anything, "bad").
					Return(nil, errors.New("not found"))
			},
			wantCode: codes.NotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := new(MockUC)
			tc.setup(uc)
			h := grpcapp.NewHandler(uc)

			resp, err := h.GetProduct(context.Background(), &pb.GetProductRequest{Id: tc.id})

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, resp.GetProduct().GetId())
			uc.AssertExpectations(t)
		})
	}
}

func TestHandler_DeductStock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		req         *pb.DeductStockRequest
		setup       func(*MockUC)
		wantCode    codes.Code
		wantSuccess bool
		wantBatch   string
	}{
		{
			name: "success",
			req:  &pb.DeductStockRequest{ProductId: "p1", Quantity: 10, OrderId: "ord-1"},
			setup: func(m *MockUC) {
				m.On("DeductStock", mock.Anything, "p1", 10, "ord-1").
					Return("batch-42", float64(150.5), "Aspirin", "анальгетики", nil)
			},
			wantSuccess: true,
			wantBatch:   "batch-42",
		},
		{
			name: "insufficient stock",
			req:  &pb.DeductStockRequest{ProductId: "p1", Quantity: 9999, OrderId: "ord-2"},
			setup: func(m *MockUC) {
				m.On("DeductStock", mock.Anything, "p1", 9999, "ord-2").
					Return("", float64(0), "", "", domain.ErrInsufficientStock)
			},
			wantCode: codes.FailedPrecondition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			uc := new(MockUC)
			tc.setup(uc)
			h := grpcapp.NewHandler(uc)

			resp, err := h.DeductStock(context.Background(), tc.req)

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantSuccess, resp.GetSuccess())
			assert.Equal(t, tc.wantBatch, resp.GetBatchId())
			uc.AssertExpectations(t)
		})
	}
}

func TestHandler_WriteOffExpired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(*MockUC)
		wantCode codes.Code
		want     int32
	}{
		{
			name: "two batches written off",
			setup: func(m *MockUC) {
				m.On("WriteOffExpired", mock.Anything).Return(3, nil)
			},
			want: 3,
		},
		{
			name: "use case error",
			setup: func(m *MockUC) {
				m.On("WriteOffExpired", mock.Anything).Return(0, errors.New("db down"))
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

			resp, err := h.WriteOffExpired(context.Background(), &pb.WriteOffExpiredRequest{})

			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, resp.GetWrittenOffCount())
			uc.AssertExpectations(t)
		})
	}
}

func TestHandler_ListLowStock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(*MockUC)
		wantCode codes.Code
		wantLen  int
	}{
		{
			name: "two items",
			setup: func(m *MockUC) {
				m.On("ListLowStock", mock.Anything).Return([]*domain.StockItem{
					{ProductID: "p1", TotalQuantity: 5, Reserved: 0, ReorderPoint: 10},
					{ProductID: "p2", TotalQuantity: 1, Reserved: 0, ReorderPoint: 5},
				}, nil)
			},
			wantLen: 2,
		},
		{
			name: "use case error",
			setup: func(m *MockUC) {
				m.On("ListLowStock", mock.Anything).Return([]*domain.StockItem{}, errors.New("db down"))
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

			resp, err := h.ListLowStock(context.Background(), &pb.ListLowStockRequest{})
			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.GetItems(), tc.wantLen)
		})
	}
}
