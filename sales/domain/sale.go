package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmptyItems    = errors.New("sale must have at least one item")
	ErrInvalidQty    = errors.New("item quantity must be positive")
	ErrSaleNotFound  = errors.New("sale not found")
)

type SaleItem struct {
	ID           string
	SaleID       string
	ProductID    string
	Quantity     int
	PricePerUnit float64
	TotalPrice   float64
}

type Sale struct {
	ID          string
	Items       []SaleItem
	TotalAmount float64
	SoldAt      time.Time
}

// NewSale creates a new sale from a list of sold items.
// Each item must already have ProductID, Quantity, PricePerUnit filled in.
func NewSale(items []SaleItem) (*Sale, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}

	saleID := uuid.NewString()
	now := time.Now()

	var total float64
	for i := range items {
		if items[i].Quantity <= 0 {
			return nil, ErrInvalidQty
		}
		items[i].ID = uuid.NewString()
		items[i].SaleID = saleID
		items[i].TotalPrice = float64(items[i].Quantity) * items[i].PricePerUnit
		total += items[i].TotalPrice
	}

	return &Sale{
		ID:          saleID,
		Items:       items,
		TotalAmount: total,
		SoldAt:      now,
	}, nil
}
