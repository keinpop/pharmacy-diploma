package usecase

import (
	"context"
	"errors"
	"time"

	"pharmacy/inventory/domain"
)

// StockRepository — порт для хранения остатков.
type StockRepository interface {
	GetByProduct(ctx context.Context, productID string) (*domain.StockItem, error)
	Upsert(ctx context.Context, item *domain.StockItem) error
	ListLowStock(ctx context.Context) ([]*domain.StockItem, error)
}

// ProductRepository — порт хранилища продуктов.
type ProductRepository interface {
	Create(ctx context.Context, p *domain.Product) error
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	List(ctx context.Context, page, pageSize int) ([]*domain.Product, int, error)
	Update(ctx context.Context, p *domain.Product) error
}

// BatchRepository — порт хранилища партий.
type BatchRepository interface {
	Create(ctx context.Context, b *domain.Batch) error
	GetByID(ctx context.Context, id string) (*domain.Batch, error)
	ListByProduct(ctx context.Context, productID string) ([]*domain.Batch, error)
	ListExpiring(ctx context.Context, daysAhead int) ([]*domain.Batch, error)
	ListExpired(ctx context.Context) ([]*domain.Batch, error)
	UpdateStatus(ctx context.Context, id string, status domain.BatchStatus) error
	Save(ctx context.Context, b *domain.Batch) error
}

// SearchRepository — порт поиска через Elasticsearch.
type SearchRepository interface {
	IndexProduct(ctx context.Context, p *domain.Product) error
	Search(ctx context.Context, query string, limit int) ([]*domain.Product, error)
	SearchBySubstance(ctx context.Context, substance string, limit int) ([]*domain.Product, error)
	ReindexAll(ctx context.Context, products []*domain.Product) error
}

// CreateProductInput — входные данные для создания продукта.
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
}

// ReceiveBatchInput — входные данные для приёма партии.
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
