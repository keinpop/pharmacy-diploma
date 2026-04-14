package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/sales/domain"
)

func TestNewSale(t *testing.T) {
	tests := []struct {
		name    string
		items   []domain.SaleItem
		wantErr error
	}{
		{
			name:    "empty items",
			items:   nil,
			wantErr: domain.ErrEmptyItems,
		},
		{
			name: "zero quantity",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 0, PricePerUnit: 100},
			},
			wantErr: domain.ErrInvalidQty,
		},
		{
			name: "negative quantity",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: -1, PricePerUnit: 100},
			},
			wantErr: domain.ErrInvalidQty,
		},
		{
			name: "single item ok",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 3, PricePerUnit: 150.50},
			},
			wantErr: nil,
		},
		{
			name: "multiple items ok",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 2, PricePerUnit: 100},
				{ProductID: "p2", Quantity: 1, PricePerUnit: 200},
			},
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sale, err := domain.NewSale(tc.items)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, sale)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sale)
			assert.NotEmpty(t, sale.ID)
			assert.False(t, sale.SoldAt.IsZero())

			var expectedTotal float64
			for _, item := range sale.Items {
				assert.NotEmpty(t, item.ID)
				assert.Equal(t, sale.ID, item.SaleID)
				expectedTotal += item.TotalPrice
			}
			assert.InDelta(t, expectedTotal, sale.TotalAmount, 0.01)
		})
	}
}
