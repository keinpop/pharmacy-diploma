package grpcclient

import (
	"context"
	"fmt"

	grpcapp "pharmacy/analytics/app/grpc"
	usecase "pharmacy/analytics/domain/use_case"
	inventorypb "pharmacy/analytics/gen/inventory"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// InventoryClient wraps the gRPC client to the Inventory service.
type InventoryClient struct {
	client       inventorypb.InventoryServiceClient
	serviceToken string
}

// NewInventoryClient creates a new InventoryClient connected to the given address.
func NewInventoryClient(addr, serviceToken string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("inventory grpc client: %w", err)
	}
	return &InventoryClient{
		client:       inventorypb.NewInventoryServiceClient(conn),
		serviceToken: serviceToken,
	}, nil
}

// authCtx appends the service token to outgoing metadata.
func (c *InventoryClient) authCtx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer magic")
}

// GetStock returns total quantity and reserved quantity for a product.
func (c *InventoryClient) GetStock(ctx context.Context, productID string) (totalQty int, reserved int, err error) {
	token := grpcapp.AuthTokenFromContext(ctx)
	if token != "" {
		fmt.Println("hello, token")
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+token)
	}
	resp, err := c.client.GetStock(c.authCtx(ctx), &inventorypb.GetStockRequest{
		ProductId: productID,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("GetStock: %w", err)
	}
	if resp.Stock == nil {
		return 0, 0, nil
	}
	return int(resp.Stock.TotalQuantity), int(resp.Stock.Reserved), nil
}

// ListExpiringBatches returns batches expiring within daysAhead days.
func (c *InventoryClient) ListExpiringBatches(ctx context.Context, daysAhead int) ([]usecase.ExpiringBatch, error) {
	token := grpcapp.AuthTokenFromContext(ctx)
	if token != "" {
		fmt.Println("hello, token")
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+token)
	}
	resp, err := c.client.ListExpiringBatches(c.authCtx(ctx), &inventorypb.ListExpiringRequest{
		DaysAhead: int32(daysAhead),
	})
	if err != nil {
		return nil, fmt.Errorf("ListExpiringBatches: %w", err)
	}

	batches := make([]usecase.ExpiringBatch, 0, len(resp.Batches))
	for _, b := range resp.Batches {
		batch := usecase.ExpiringBatch{
			ProductID: b.ProductId,
			Quantity:  int(b.Quantity),
		}
		if b.ExpiresAt != nil {
			batch.ExpiresAt = b.ExpiresAt.AsTime()
		}
		batches = append(batches, batch)
	}
	return batches, nil
}

// Ensure interface compliance at compile time.
var _ usecase.InventoryClient = (*InventoryClient)(nil)
