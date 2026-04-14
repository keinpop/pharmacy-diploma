package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pharmacy/inventory/domain"
)

func TestStockItem_LowStock_True(t *testing.T) {
	s := &domain.StockItem{ProductID: "p1", TotalQuantity: 15, Reserved: 5, ReorderPoint: 10}
	assert.True(t, s.LowStock()) // available=10, reorder=10
}

func TestStockItem_LowStock_False(t *testing.T) {
	s := &domain.StockItem{ProductID: "p1", TotalQuantity: 50, Reserved: 5, ReorderPoint: 10}
	assert.False(t, s.LowStock())
}
