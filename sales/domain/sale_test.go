package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/sales/domain"
)

func TestNewSale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		items   []domain.SaleItem
		seller  string
		wantErr error
	}{
		{
			name:    "пустой список позиций",
			items:   nil,
			seller:  "alice",
			wantErr: domain.ErrEmptyItems,
		},
		{
			name:    "отсутствует продавец",
			items:   []domain.SaleItem{{ProductID: "p1", Quantity: 1, PricePerUnit: 100}},
			seller:  "",
			wantErr: domain.ErrEmptySeller,
		},
		{
			name:    "продавец из пробелов",
			items:   []domain.SaleItem{{ProductID: "p1", Quantity: 1, PricePerUnit: 100}},
			seller:  "   ",
			wantErr: domain.ErrEmptySeller,
		},
		{
			name: "нулевое количество",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 0, PricePerUnit: 100},
			},
			seller:  "alice",
			wantErr: domain.ErrInvalidQty,
		},
		{
			name: "отрицательное количество",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: -1, PricePerUnit: 100},
			},
			seller:  "alice",
			wantErr: domain.ErrInvalidQty,
		},
		{
			name: "одна позиция, валидно",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 3, PricePerUnit: 150.50},
			},
			seller: "alice",
		},
		{
			name: "несколько позиций, валидно",
			items: []domain.SaleItem{
				{ProductID: "p1", Quantity: 2, PricePerUnit: 100},
				{ProductID: "p2", Quantity: 1, PricePerUnit: 200},
			},
			seller: "bob",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sale, err := domain.NewSale(tc.items, tc.seller)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, sale)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sale)
			assert.NotEmpty(t, sale.ID)
			assert.Equal(t, tc.seller, sale.SellerUsername)
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
