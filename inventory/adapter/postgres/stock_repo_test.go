package postgres_test

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pharmacy/inventory/adapter/postgres"
	"pharmacy/inventory/domain"
)

func newStockRepo(t *testing.T) (*postgres.StockRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return postgres.NewStockRepository(db), mock
}

var stockCols = []string{"product_id", "total_quantity", "reserved", "reorder_point"}

//  GetByProduct

func TestStockRepository_GetByProduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		productID    string
		setup        func(sqlmock.Sqlmock)
		wantTotal    int
		wantReserved int
		wantErr      bool
	}{
		{
			name:      "found",
			productID: "p1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WithArgs("p1").
					WillReturnRows(sqlmock.NewRows(stockCols).AddRow("p1", 100, 10, 15))
			},
			wantTotal:    100,
			wantReserved: 10,
		},
		{
			name:      "not found — empty StockItem, no error",
			productID: "p2",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WithArgs("p2").
					WillReturnRows(sqlmock.NewRows(stockCols))
			},
			wantTotal:    0,
			wantReserved: 0,
		},
		{
			name:      "db error",
			productID: "p3",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WithArgs("p3").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newStockRepo(t)
			tc.setup(mock)

			result, err := repo.GetByProduct(context.Background(), tc.productID)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.productID, result.ProductID)
				assert.Equal(t, tc.wantTotal, result.TotalQuantity)
				assert.Equal(t, tc.wantReserved, result.Reserved)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  Upsert

func TestStockRepository_Upsert(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		stock   *domain.StockItem
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:  "insert new",
			stock: &domain.StockItem{ProductID: "p1", TotalQuantity: 100, Reserved: 5, ReorderPoint: 10},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO stock_items`)).
					WithArgs("p1", 100, 5, 10).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:  "update on conflict",
			stock: &domain.StockItem{ProductID: "p1", TotalQuantity: 80, Reserved: 0, ReorderPoint: 10},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO stock_items`)).
					WithArgs("p1", 80, 0, 10).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:  "db error",
			stock: &domain.StockItem{ProductID: "p3"},
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO stock_items`)).
					WithArgs("p3", 0, 0, 0).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newStockRepo(t)
			tc.setup(mock)

			err := repo.Upsert(context.Background(), tc.stock)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  ListLowStock

func TestStockRepository_ListLowStock(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func(sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "two low stock items",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WillReturnRows(sqlmock.NewRows(stockCols).
						AddRow("p1", 10, 5, 10).
						AddRow("p2", 8, 0, 10))
			},
			wantCount: 2,
		},
		{
			name: "no low stock items",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WillReturnRows(sqlmock.NewRows(stockCols))
			},
			wantCount: 0,
		},
		{
			name: "db error",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, total_quantity`)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newStockRepo(t)
			tc.setup(mock)

			result, err := repo.ListLowStock(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
				for _, s := range result {
					assert.True(t, s.Available() <= s.ReorderPoint)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
