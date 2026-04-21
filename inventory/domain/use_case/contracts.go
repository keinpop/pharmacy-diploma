package usecase

import (
	"context"
	"errors"
	"time"

	"pharmacy/inventory/domain"
)

// BatchRepository — порт для хранения партий.
type BatchRepository interface {
	Create(ctx context.Context, b *domain.Batch) error
	Save(ctx context.Context, b *domain.Batch) error
	ListByProduct(ctx context.Context, productID string) ([]*domain.Batch, error)
	ListExpiring(ctx context.Context, daysAhead int) ([]*domain.Batch, error)
	ListExpired(ctx context.Context) ([]*domain.Batch, error)
	UpdateStatus(ctx context.Context, id string, status domain.BatchStatus) error
}

// StockRepository — порт для хранения остатков.
type StockRepository interface {
	GetByProduct(ctx context.Context, productID string) (*domain.StockItem, error)
	Upsert(ctx context.Context, item *domain.StockItem) error
	ListLowStock(ctx context.Context) ([]*domain.StockItem, error)
}

// ProductRepository — порт для хранения продуктов.
type ProductRepository interface {
	Create(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
}

// SearchRepository — порт для полнотекстового поиска.
type SearchRepository interface {
	IndexProduct(ctx context.Context, p *domain.Product) error
	Search(ctx context.Context, query string, limit int) ([]*domain.Product, error)
	SearchBySubstance(ctx context.Context, substance string, limit int) ([]*domain.Product, error)
	ReindexAll(ctx context.Context, products []*domain.Product) error
}

// EventPublisher — порт для публикации событий в Kafka.
type EventPublisher interface {
	PublishBatchReceived(ctx context.Context, event BatchReceivedEvent) error
	PublishBatchWrittenOff(ctx context.Context, event BatchWrittenOffEvent) error
}

// BatchReceivedEvent — событие получения партии товара.
type BatchReceivedEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	BatchID          string    `json:"batch_id"`
	Quantity         int       `json:"quantity"`
	RetailPrice      float64   `json:"retail_price"`
	ExpiresAt        time.Time `json:"expires_at"`
	ReceivedAt       time.Time `json:"received_at"`
}

// BatchWrittenOffEvent — событие списания партии товара.
type BatchWrittenOffEvent struct {
	ProductID        string    `json:"product_id"`
	ProductName      string    `json:"product_name"`
	TherapeuticGroup string    `json:"therapeutic_group"`
	BatchID          string    `json:"batch_id"`
	Quantity         int       `json:"quantity"`
	ExpiresAt        time.Time `json:"expires_at"`
	WrittenOffAt     time.Time `json:"written_off_at"`
}

// Input structs

type CreateProductInput struct {
	Name              string
	TradeName         string
	ActiveSubstance   string
	Form              string
	Dosage            string
	Category          domain.Category
	StorageConditions string
	Unit              string
	ReorderPoint      int
	TherapeuticGroup  string
}

type ReceiveBatchInput struct {
	ProductID     string
	SeriesNumber  string
	ExpiresAt     time.Time
	Quantity      int
	PurchasePrice float64
	RetailPrice   float64
}

var (
	ErrProductNotFound      = errors.New("product not found")
	ErrBatchNotFound        = errors.New("batch not found")
	ErrEmptyProductName     = errors.New("empty product name")
	ErrEmptyActiveSubstance = errors.New("empty active substance")
	ErrInvalidCategory      = errors.New("invalid category")
	ErrInvalidQuantity      = errors.New("invalid quantity")
	ErrBatchAlreadyExpired  = errors.New("batch already expired")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrEmptySearchQuery     = errors.New("empty search query")
)
