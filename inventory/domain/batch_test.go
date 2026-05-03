package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/inventory/domain"
)

func TestNewBatch(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name             string
		productID        string
		series           string
		expiresAt        time.Time
		quantity         int
		expiringSoonDays int
		wantStatus       domain.BatchStatus
		wantErr          error
	}{
		{
			name:             "available — срок далеко",
			productID:        "p1",
			series:           "SER-001",
			expiresAt:        now.AddDate(0, 6, 0),
			quantity:         100,
			expiringSoonDays: 30,
			wantStatus:       domain.BatchStatusAvailable,
		},
		{
			name:             "expiring_soon — до конца окна меньше N дней",
			productID:        "p1",
			series:           "SER-002",
			expiresAt:        now.AddDate(0, 0, 10),
			quantity:         50,
			expiringSoonDays: 30,
			wantStatus:       domain.BatchStatusExpiringSoon,
		},
		{
			name:             "expired — дата в прошлом, конструктор должен ругаться",
			productID:        "p1",
			series:           "SER-003",
			expiresAt:        now.AddDate(0, 0, -1),
			quantity:         50,
			expiringSoonDays: 30,
			wantErr:          domain.ErrBatchAlreadyExpired,
		},
		{
			name:             "нулевое количество",
			productID:        "p1",
			series:           "SER-004",
			expiresAt:        now.AddDate(0, 6, 0),
			quantity:         0,
			expiringSoonDays: 30,
			wantErr:          domain.ErrInvalidQuantity,
		},
		{
			name:             "отрицательное количество",
			productID:        "p1",
			series:           "SER-005",
			expiresAt:        now.AddDate(0, 6, 0),
			quantity:         -5,
			expiringSoonDays: 30,
			wantErr:          domain.ErrInvalidQuantity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBatch(tc.productID, tc.series, tc.expiresAt,
				tc.quantity, 10, 15, tc.expiringSoonDays)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
				assert.Nil(t, b)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, b)
			assert.NotEmpty(t, b.ID)
			assert.Equal(t, tc.productID, b.ProductID)
			assert.Equal(t, tc.quantity, b.Quantity)
			assert.Equal(t, tc.wantStatus, b.Status)
			assert.Equal(t, tc.quantity, b.Available())
		})
	}
}

func TestBatch_Deduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		quantity      int
		preReserved   int
		deduct        int
		wantReserved  int
		wantAvailable int
		wantErr       error
	}{
		{
			name:          "успешное списание",
			quantity:      10,
			deduct:        5,
			wantReserved:  5,
			wantAvailable: 5,
		},
		{
			name:          "списание ровно всего остатка",
			quantity:      10,
			preReserved:   2,
			deduct:        8,
			wantReserved:  10,
			wantAvailable: 0,
		},
		{
			name:     "недостаточно остатков",
			quantity: 10,
			deduct:   100,
			wantErr:  domain.ErrInsufficientStock,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, err := domain.NewBatch("p1", "SER", time.Now().AddDate(0, 6, 0), tc.quantity, 10, 15, 30)
			require.NoError(t, err)
			b.Reserved = tc.preReserved

			err = b.Deduct(tc.deduct)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantReserved, b.Reserved)
			assert.Equal(t, tc.wantAvailable, b.Available())
		})
	}
}
