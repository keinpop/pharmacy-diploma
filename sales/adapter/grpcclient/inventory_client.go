package grpcclient

import (
	"context"
	"fmt"

	grpcapp "pharmacy/sales/app/grpc"
	inventorypb "pharmacy/sales/gen/inventory"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type InventoryClient struct {
	client inventorypb.InventoryServiceClient
}

func NewInventoryClient(addr string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("inventory client: %w", err)
	}
	return &InventoryClient{client: inventorypb.NewInventoryServiceClient(conn)}, nil
}

func (c *InventoryClient) DeductStock(ctx context.Context, productID string, qty int, orderID string) (string, float64, error) {
	token := grpcapp.AuthTokenFromContext(ctx)
	if token != "" {
		fmt.Println("hello, token")
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+token)
	}

	resp, err := c.client.DeductStock(ctx, &inventorypb.DeductStockRequest{
		ProductId: productID,
		Quantity:  int32(qty),
		OrderId:   orderID,
	})
	if err != nil {
		return "", 0, err
	}
	return resp.BatchId, resp.RetailPrice, nil
}
