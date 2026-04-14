package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/sales/adapter/postgres"
	"pharmacy/sales/domain"
)

func TestSaleRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := postgres.NewSaleRepository(db)

	sale := &domain.Sale{
		ID:          "sale-1",
		TotalAmount: 450.50,
		SoldAt:      time.Now(),
		Items: []domain.SaleItem{
			{ID: "item-1", SaleID: "sale-1", ProductID: "p1", Quantity: 3, PricePerUnit: 150.50, TotalPrice: 451.50},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sales").
		WithArgs(sale.ID, sale.TotalAmount, sale.SoldAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO sale_items").
		WithArgs("item-1", "sale-1", "p1", 3, 150.50, 451.50).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.Create(context.Background(), sale)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSaleRepository_GetByID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		setup   func(m sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "found",
			id:   "sale-1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, total_amount, sold_at FROM sales").
					WithArgs("sale-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "total_amount", "sold_at"}).
						AddRow("sale-1", 300.0, time.Now()))
				m.ExpectQuery("SELECT id, sale_id, product_id, quantity, price_per_unit, total_price").
					WithArgs("sale-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "sale_id", "product_id", "quantity", "price_per_unit", "total_price"}).
						AddRow("item-1", "sale-1", "p1", 2, 150.0, 300.0))
			},
			wantErr: false,
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery("SELECT id, total_amount, sold_at FROM sales").
					WithArgs("bad").
					WillReturnRows(sqlmock.NewRows([]string{"id", "total_amount", "sold_at"}))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, m, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tc.setup(m)
			repo := postgres.NewSaleRepository(db)
			sale, err := repo.GetByID(context.Background(), tc.id)

			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, sale)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.id, sale.ID)
			assert.NoError(t, m.ExpectationsWereMet())
		})
	}
}
