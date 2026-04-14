package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type BatchStatus string

const (
	BatchStatusAvailable    BatchStatus = "available"
	BatchStatusExpiringSoon BatchStatus = "expiring_soon"
	BatchStatusExpired      BatchStatus = "expired"
	BatchStatusWrittenOff   BatchStatus = "written_off"
)

var (
	ErrInvalidQuantity     = errors.New("quantity must be positive")
	ErrBatchAlreadyExpired = errors.New("batch expiration date is in the past")
)

type Batch struct {
	ID            string
	ProductID     string
	SeriesNumber  string    // серийный номер производителя
	ExpiresAt     time.Time // срок годности
	ReceivedAt    time.Time // дата поступления
	Quantity      int       // общее количество
	Reserved      int       // зарезервировано (заказы в обработке)
	PurchasePrice float64
	RetailPrice   float64
	Status        BatchStatus
}

func NewBatch(
	productID, seriesNumber string,
	expiresAt time.Time,
	quantity int,
	purchasePrice, retailPrice float64,
	expiringSoonDays int,
) (*Batch, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	now := time.Now()
	if expiresAt.Before(now) {
		return nil, ErrBatchAlreadyExpired
	}
	status := BatchStatusAvailable
	if expiresAt.Before(now.AddDate(0, 0, expiringSoonDays)) {
		status = BatchStatusExpiringSoon
	}
	return &Batch{
		ID:            uuid.NewString(),
		ProductID:     productID,
		SeriesNumber:  seriesNumber,
		ExpiresAt:     expiresAt,
		ReceivedAt:    now,
		Quantity:      quantity,
		Reserved:      0,
		PurchasePrice: purchasePrice,
		RetailPrice:   retailPrice,
		Status:        status,
	}, nil
}

func (b *Batch) Available() int {
	return b.Quantity - b.Reserved
}

func (b *Batch) Deduct(qty int) error {
	if qty > b.Available() {
		return ErrInsufficientStock
	}
	b.Reserved += qty
	return nil
}
