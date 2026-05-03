package usecase

import (
	"context"
	"time"

	"pharmacy/sales/domain"
)

// SaleRepository — порт для хранения продаж.
type SaleRepository interface {
	Create(ctx context.Context, sale *domain.Sale) error
	GetByID(ctx context.Context, id string) (*domain.Sale, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.Sale, int, error)
}

// InventoryClient — порт для взаимодействия с Inventory сервисом.
type InventoryClient interface {
	DeductStock(ctx context.Context, productID string, qty int, orderID string) (batchID string, retailPrice float64, productName string, therapeuticGroup string, err error)
}

// EventPublisher — порт для публикации событий в Kafka.
type EventPublisher interface {
	PublishSaleCompleted(ctx context.Context, event SaleCompletedEvent) error
}

// SaleCompletedEvent — событие завершения продажи.
type SaleCompletedEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	Quantity         int       `json:"quantity"`
	PricePerUnit     float64   `json:"price_per_unit"`
	TotalPrice       float64   `json:"total_price"`
	SoldAt           time.Time `json:"sold_at"`
	SellerUsername   string    `json:"seller_username"`
}

type CreateSaleItemInput struct {
	ProductID string
	Quantity  int
}

type CreateSaleInput struct {
	Items []CreateSaleItemInput
}
