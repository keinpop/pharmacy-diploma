package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pharmacy/inventory/domain"
)

func TestNewProduct_Valid(t *testing.T) {
	p, err := domain.NewProduct("Аспирин", "Aspirin Cardio", "ацетилсалициловая кислота",
		"таблетка", "100мг", domain.CategoryOTC, "комнатная температура", "таблетка", 20)
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "Аспирин", p.Name)
}

func TestNewProduct_MissingName(t *testing.T) {
	_, err := domain.NewProduct("", "Trade", "substance", "tablet", "100mg",
		domain.CategoryOTC, "", "tablet", 10)
	assert.Error(t, err)
}

func TestNewProduct_MissingSubstance(t *testing.T) {
	_, err := domain.NewProduct("Aspirin", "Trade", "", "tablet", "100mg",
		domain.CategoryOTC, "", "tablet", 10)
	assert.Error(t, err)
}
