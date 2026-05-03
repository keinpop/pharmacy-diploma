package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pharmacy/inventory/domain"
)

func TestStockItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		total         int
		reserved      int
		reorderPoint  int
		wantAvailable int
		wantLowStock  bool
	}{
		{
			name:  "low stock — equal to reorder point",
			total: 15, reserved: 5, reorderPoint: 10,
			wantAvailable: 10, wantLowStock: true,
		},
		{
			name:  "low stock — ниже точки заказа",
			total: 8, reserved: 0, reorderPoint: 10,
			wantAvailable: 8, wantLowStock: true,
		},
		{
			name:  "запас в норме",
			total: 50, reserved: 5, reorderPoint: 10,
			wantAvailable: 45, wantLowStock: false,
		},
		{
			name:  "товар полностью зарезервирован",
			total: 10, reserved: 10, reorderPoint: 5,
			wantAvailable: 0, wantLowStock: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &domain.StockItem{
				ProductID:     "p1",
				TotalQuantity: tc.total,
				Reserved:      tc.reserved,
				ReorderPoint:  tc.reorderPoint,
			}
			assert.Equal(t, tc.wantAvailable, s.Available())
			assert.Equal(t, tc.wantLowStock, s.LowStock())
		})
	}
}
