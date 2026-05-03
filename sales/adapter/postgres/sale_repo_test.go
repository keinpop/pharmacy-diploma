package postgres_test

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/sales/adapter/postgres"
	"pharmacy/sales/domain"
)

func newSaleRepo(t *testing.T) (*postgres.SaleRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return postgres.NewSaleRepository(db), mock
}

func TestSaleRepository_Create(t *testing.T) {
	t.Parallel()
	now := time.Now()
	sale := &domain.Sale{
		ID:             "sale-1",
		TotalAmount:    451.50,
		SoldAt:         now,
		SellerUsername: "alice",
		Items: []domain.SaleItem{
			{ID: "item-1", SaleID: "sale-1", ProductID: "p1", Quantity: 3, PricePerUnit: 150.50, TotalPrice: 451.50},
		},
	}

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO sales`)).
					WithArgs(sale.ID, sale.TotalAmount, sale.SoldAt, sale.SellerUsername).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO sale_items`)).
					WithArgs("item-1", "sale-1", "p1", 3, 150.50, 451.50).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			},
		},
		{
			name: "insert sale fails",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO sales`)).
					WithArgs(sale.ID, sale.TotalAmount, sale.SoldAt, sale.SellerUsername).
					WillReturnError(sql.ErrConnDone)
				m.ExpectRollback()
			},
			wantErr: true,
		},
		{
			name: "insert item fails",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO sales`)).
					WithArgs(sale.ID, sale.TotalAmount, sale.SoldAt, sale.SellerUsername).
					WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO sale_items`)).
					WithArgs("item-1", "sale-1", "p1", 3, 150.50, 451.50).
					WillReturnError(sql.ErrConnDone)
				m.ExpectRollback()
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newSaleRepo(t)
			tc.setup(mock)

			err := repo.Create(context.Background(), sale)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSaleRepository_GetByID(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name          string
		id            string
		setup         func(m sqlmock.Sqlmock)
		wantSeller    string
		wantItemCount int
		wantErr       error
		expectNilSale bool
	}{
		{
			name: "found",
			id:   "sale-1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, total_amount, sold_at, COALESCE(seller_username, '')`)).
					WithArgs("sale-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "total_amount", "sold_at", "seller_username"}).
						AddRow("sale-1", 300.0, now, "alice"))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, sale_id, product_id, quantity, price_per_unit, total_price`)).
					WithArgs("sale-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "sale_id", "product_id", "quantity", "price_per_unit", "total_price"}).
						AddRow("item-1", "sale-1", "p1", 2, 150.0, 300.0))
			},
			wantSeller:    "alice",
			wantItemCount: 1,
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, total_amount, sold_at, COALESCE(seller_username, '')`)).
					WithArgs("bad").
					WillReturnRows(sqlmock.NewRows([]string{"id", "total_amount", "sold_at", "seller_username"}))
			},
			wantErr:       domain.ErrSaleNotFound,
			expectNilSale: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newSaleRepo(t)
			tc.setup(mock)

			sale, err := repo.GetByID(context.Background(), tc.id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				if tc.expectNilSale {
					assert.Nil(t, sale)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sale)
			assert.Equal(t, tc.id, sale.ID)
			assert.Equal(t, tc.wantSeller, sale.SellerUsername)
			assert.Len(t, sale.Items, tc.wantItemCount)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSaleRepository_List(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name      string
		page      int
		pageSize  int
		setup     func(sqlmock.Sqlmock)
		wantCount int
		wantTotal int
		wantErr   bool
	}{
		{
			name: "first page",
			page: 1, pageSize: 20,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM sales`)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, total_amount, sold_at, COALESCE(seller_username, '')`)).
					WithArgs(20, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "total_amount", "sold_at", "seller_username"}).
						AddRow("sale-1", 100.0, now, "alice").
						AddRow("sale-2", 200.0, now, "bob"))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, sale_id, product_id, quantity, price_per_unit, total_price`)).
					WithArgs("sale-1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "sale_id", "product_id", "quantity", "price_per_unit", "total_price"}))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, sale_id, product_id, quantity, price_per_unit, total_price`)).
					WithArgs("sale-2").
					WillReturnRows(sqlmock.NewRows([]string{"id", "sale_id", "product_id", "quantity", "price_per_unit", "total_price"}))
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name: "count error",
			page: 1, pageSize: 20,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM sales`)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newSaleRepo(t)
			tc.setup(mock)

			sales, total, err := repo.List(context.Background(), tc.page, tc.pageSize)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, sales, tc.wantCount)
			assert.Equal(t, tc.wantTotal, total)
		})
	}
}
