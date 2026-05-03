package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/inventory/domain"
)

func TestNewProduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		productName     string
		tradeName       string
		activeSubstance string
		category        domain.Category
		wantErr         error
	}{
		{
			name:            "валидный безрецептурный препарат",
			productName:     "Аспирин",
			tradeName:       "Aspirin Cardio",
			activeSubstance: "ацетилсалициловая кислота",
			category:        domain.CategoryOTC,
		},
		{
			name:            "валидный рецептурный препарат",
			productName:     "Амоксиклав",
			activeSubstance: "амоксициллин",
			category:        domain.CategoryPrescription,
		},
		{
			name:            "пустое название",
			productName:     "",
			activeSubstance: "сабстанция",
			category:        domain.CategoryOTC,
			wantErr:         domain.ErrEmptyProductName,
		},
		{
			name:            "пустое действующее вещество",
			productName:     "Аспирин",
			activeSubstance: "",
			category:        domain.CategoryOTC,
			wantErr:         domain.ErrEmptyActiveSubstance,
		},
		{
			name:            "невалидная категория",
			productName:     "Аспирин",
			activeSubstance: "сабстанция",
			category:        "unknown",
			wantErr:         domain.ErrInvalidCategory,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := domain.NewProduct(
				tc.productName, tc.tradeName, tc.activeSubstance,
				"таблетка", "100мг",
				tc.category,
				"комнатная температура", "таблетка",
				20, "терапия",
			)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, p)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, p)
			assert.NotEmpty(t, p.ID)
			assert.Equal(t, tc.productName, p.Name)
			assert.Equal(t, tc.activeSubstance, p.ActiveSubstance)
			assert.Equal(t, tc.category, p.Category)
		})
	}
}
