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

	"pharmacy/inventory/adapter/postgres"
	"pharmacy/inventory/domain"
	usecase "pharmacy/inventory/domain/use_case"
)

func newProductRepo(t *testing.T) (*postgres.ProductRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return postgres.NewProductRepository(db), mock
}

// Те же колонки, что возвращает реальный SELECT в репозитории.
var productCols = []string{
	"id", "name", "trade_name", "active_substance", "form", "dosage",
	"category", "storage_conditions", "unit", "therapeutic_group",
	"reorder_point", "created_at", "updated_at",
}

func TestProductRepository_Create(t *testing.T) {
	t.Parallel()
	now := time.Now()
	validProduct := &domain.Product{
		ID: "p1", Name: "Аспирин", TradeName: "Aspirin Cardio",
		ActiveSubstance: "аск", Form: "таблетка", Dosage: "100мг",
		Category: domain.CategoryOTC, Unit: "таблетка", ReorderPoint: 10,
		TherapeuticGroup: "анальгетики",
		CreatedAt:        now, UpdatedAt: now,
	}

	tests := []struct {
		name    string
		product *domain.Product
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:    "success",
			product: validProduct,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO products`)).
					WithArgs(
						validProduct.ID, validProduct.Name, validProduct.TradeName,
						validProduct.ActiveSubstance, validProduct.Form, validProduct.Dosage,
						string(validProduct.Category), validProduct.StorageConditions,
						validProduct.Unit, validProduct.TherapeuticGroup,
						validProduct.ReorderPoint, sqlmock.AnyArg(), sqlmock.AnyArg(),
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name:    "db error",
			product: validProduct,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO products`)).
					WithArgs(
						validProduct.ID, validProduct.Name, validProduct.TradeName,
						validProduct.ActiveSubstance, validProduct.Form, validProduct.Dosage,
						string(validProduct.Category), validProduct.StorageConditions,
						validProduct.Unit, validProduct.TherapeuticGroup,
						validProduct.ReorderPoint, sqlmock.AnyArg(), sqlmock.AnyArg(),
					).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newProductRepo(t)
			tc.setup(mock)

			err := repo.Create(context.Background(), tc.product)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductRepository_GetByID(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name    string
		id      string
		setup   func(sqlmock.Sqlmock)
		wantErr error
		wantID  string
	}{
		{
			name: "found",
			id:   "p1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs("p1").
					WillReturnRows(sqlmock.NewRows(productCols).AddRow(
						"p1", "Аспирин", "Aspirin Cardio", "аск", "таблетка", "100мг",
						"otc", "", "таблетка", "анальгетики", 10, now, now,
					))
			},
			wantID: "p1",
		},
		{
			name: "not found",
			id:   "unknown",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs("unknown").
					WillReturnRows(sqlmock.NewRows(productCols))
			},
			wantErr: usecase.ErrProductNotFound,
		},
		{
			name: "db error",
			id:   "p1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs("p1").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newProductRepo(t)
			tc.setup(mock)

			result, err := repo.GetByID(context.Background(), tc.id)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantID, result.ID)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductRepository_List(t *testing.T) {
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
			name: "first page with results",
			page: 1, pageSize: 20,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs(20, 0).
					WillReturnRows(sqlmock.NewRows(productCols).
						AddRow("p1", "Аспирин", "Aspirin", "аск", "таблетка", "100мг", "otc", "", "таблетка", "анальгетики", 10, now, now).
						AddRow("p2", "Ибупрофен", "Нурофен", "ибу", "таблетка", "200мг", "otc", "", "таблетка", "анальгетики", 5, now, now))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM products`)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name: "empty page",
			page: 2, pageSize: 20,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs(20, 20).
					WillReturnRows(sqlmock.NewRows(productCols))
				m.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM products`)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
			},
			wantCount: 0,
			wantTotal: 2,
		},
		{
			name: "db error on query",
			page: 1, pageSize: 20,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, name`)).
					WithArgs(20, 0).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newProductRepo(t)
			tc.setup(mock)

			result, total, err := repo.List(context.Background(), tc.page, tc.pageSize)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
				assert.Equal(t, tc.wantTotal, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductRepository_Update(t *testing.T) {
	t.Parallel()
	now := time.Now()
	validProduct := &domain.Product{
		ID: "p1", Name: "Аспирин Updated", TradeName: "Aspirin",
		ActiveSubstance: "аск", Form: "таблетка", Dosage: "100мг",
		Category: domain.CategoryOTC, Unit: "таблетка", ReorderPoint: 10,
		TherapeuticGroup: "анальгетики",
		UpdatedAt:        now,
	}

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE products SET`)).
					WithArgs(
						validProduct.ID, validProduct.Name, validProduct.TradeName,
						validProduct.ActiveSubstance, validProduct.Form, validProduct.Dosage,
						string(validProduct.Category), validProduct.StorageConditions,
						validProduct.Unit, validProduct.TherapeuticGroup,
						validProduct.ReorderPoint, sqlmock.AnyArg(),
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "db error",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE products SET`)).
					WithArgs(
						validProduct.ID, validProduct.Name, validProduct.TradeName,
						validProduct.ActiveSubstance, validProduct.Form, validProduct.Dosage,
						string(validProduct.Category), validProduct.StorageConditions,
						validProduct.Unit, validProduct.TherapeuticGroup,
						validProduct.ReorderPoint, sqlmock.AnyArg(),
					).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newProductRepo(t)
			tc.setup(mock)

			err := repo.Update(context.Background(), validProduct)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
