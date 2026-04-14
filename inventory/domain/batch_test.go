package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pharmacy/inventory/domain"
)

func TestNewBatch_Available(t *testing.T) {
	b, err := domain.NewBatch("p1", "SER-001", time.Now().AddDate(0, 6, 0), 100, 10, 15, 30)
	require.NoError(t, err)
	assert.Equal(t, domain.BatchStatusAvailable, b.Status)
	assert.Equal(t, 100, b.Available())
}

func TestNewBatch_ExpiringSoon(t *testing.T) {
	b, err := domain.NewBatch("p1", "SER-002", time.Now().AddDate(0, 0, 10), 50, 10, 15, 30)
	require.NoError(t, err)
	assert.Equal(t, domain.BatchStatusExpiringSoon, b.Status)
}

func TestNewBatch_Expired(t *testing.T) {
	b, err := domain.NewBatch("p1", "SER-003", time.Now().AddDate(0, 0, -1), 50, 10, 15, 30)
	require.NoError(t, err)
	assert.Equal(t, domain.BatchStatusExpired, b.Status)
}

func TestBatch_Deduct_OK(t *testing.T) {
	b, _ := domain.NewBatch("p1", "SER-004", time.Now().AddDate(0, 6, 0), 10, 10, 15, 30)
	require.NoError(t, b.Deduct(5))
	assert.Equal(t, 5, b.Quantity)
}

func TestBatch_Deduct_InsufficientFails(t *testing.T) {
	b, _ := domain.NewBatch("p1", "SER-005", time.Now().AddDate(0, 6, 0), 10, 10, 15, 30)
	assert.Error(t, b.Deduct(100))
}

func TestNewBatch_ZeroQtyFails(t *testing.T) {
	_, err := domain.NewBatch("p1", "SER-006", time.Now().AddDate(0, 6, 0), 0, 10, 15, 30)
	assert.Error(t, err)
}
