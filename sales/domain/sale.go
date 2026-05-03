package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmptyItems   = errors.New("sale must have at least one item")
	ErrInvalidQty   = errors.New("item quantity must be positive")
	ErrSaleNotFound = errors.New("sale not found")
	ErrEmptySeller  = errors.New("seller username is required")
)

type SaleItem struct {
	ID           string
	SaleID       string
	ProductID    string
	Quantity     int
	PricePerUnit float64
	TotalPrice   float64
}

// Sale — агрегат продажи. Хранит, в том числе, имя пользователя,
// оформившего продажу (фармацевт / админ), что нужно для аудита и аналитики.
type Sale struct {
	ID             string
	Items          []SaleItem
	TotalAmount    float64
	SoldAt         time.Time
	SellerUsername string
}

// NewSale создаёт новую продажу из списка позиций.
// Каждая позиция должна иметь заполненные ProductID, Quantity, PricePerUnit.
// sellerUsername — обязательный параметр (берётся из JWT-токена в use-case).
func NewSale(items []SaleItem, sellerUsername string) (*Sale, error) {
	if len(items) == 0 {
		return nil, ErrEmptyItems
	}
	if strings.TrimSpace(sellerUsername) == "" {
		return nil, ErrEmptySeller
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
		ID:             saleID,
		Items:          items,
		TotalAmount:    total,
		SoldAt:         now,
		SellerUsername: sellerUsername,
	}, nil
}
