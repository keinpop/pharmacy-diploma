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

func newBatchRepo(t *testing.T) (*postgres.BatchRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return postgres.NewBatchRepository(db), mock
}

var batchCols = []string{
	"id", "product_id", "series_number", "expires_at", "received_at",
	"quantity", "reserved", "purchase_price", "retail_price", "status",
}

func newBatch(id, productID string, expiresAt time.Time, qty int, status domain.BatchStatus) *domain.Batch {
	return &domain.Batch{
		ID: id, ProductID: productID, SeriesNumber: "SER-" + id,
		ExpiresAt: expiresAt, ReceivedAt: time.Now(),
		Quantity: qty, Reserved: 0,
		PurchasePrice: 100.0, RetailPrice: 150.0, Status: status,
	}
}

//  Create

func TestBatchRepository_Create(t *testing.T) {
	t.Parallel()
	b := newBatch("b1", "p1", time.Now().AddDate(0, 6, 0), 100, domain.BatchStatusAvailable)

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO batches`)).
					WithArgs(
						b.ID, b.ProductID, b.SeriesNumber,
						sqlmock.AnyArg(), sqlmock.AnyArg(),
						b.Quantity, b.Reserved, b.PurchasePrice, b.RetailPrice, string(b.Status),
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "db error",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`INSERT INTO batches`)).
					WithArgs(
						b.ID, b.ProductID, b.SeriesNumber,
						sqlmock.AnyArg(), sqlmock.AnyArg(),
						b.Quantity, b.Reserved, b.PurchasePrice, b.RetailPrice, string(b.Status),
					).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			err := repo.Create(context.Background(), b)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  GetByID─

func TestBatchRepository_GetByID(t *testing.T) {
	t.Parallel()
	b := newBatch("b1", "p1", time.Now().AddDate(0, 6, 0), 100, domain.BatchStatusAvailable)

	tests := []struct {
		name    string
		id      string
		setup   func(sqlmock.Sqlmock)
		wantErr error
		wantID  string
	}{
		{
			name: "found",
			id:   "b1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, product_id`)).
					WithArgs("b1").
					WillReturnRows(sqlmock.NewRows(batchCols).AddRow(
						b.ID, b.ProductID, b.SeriesNumber, b.ExpiresAt, b.ReceivedAt,
						b.Quantity, b.Reserved, b.PurchasePrice, b.RetailPrice, string(b.Status),
					))
			},
			wantID: "b1",
		},
		{
			name: "not found",
			id:   "bad",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, product_id`)).
					WithArgs("bad").
					WillReturnRows(sqlmock.NewRows(batchCols))
			},
			wantErr: usecase.ErrBatchNotFound,
		},
		{
			name: "db error",
			id:   "b1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`SELECT id, product_id`)).
					WithArgs("b1").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: sql.ErrConnDone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
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

//  ListByProduct

func TestBatchRepository_ListByProduct(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b1 := newBatch("b1", "p1", now.AddDate(0, 1, 0), 50, domain.BatchStatusExpiringSoon)
	b2 := newBatch("b2", "p1", now.AddDate(0, 6, 0), 100, domain.BatchStatusAvailable)

	tests := []struct {
		name      string
		productID string
		setup     func(sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "two batches",
			productID: "p1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE product_id=$1`)).
					WithArgs("p1").
					WillReturnRows(sqlmock.NewRows(batchCols).
						AddRow(b1.ID, b1.ProductID, b1.SeriesNumber, b1.ExpiresAt, b1.ReceivedAt,
							b1.Quantity, b1.Reserved, b1.PurchasePrice, b1.RetailPrice, string(b1.Status)).
						AddRow(b2.ID, b2.ProductID, b2.SeriesNumber, b2.ExpiresAt, b2.ReceivedAt,
							b2.Quantity, b2.Reserved, b2.PurchasePrice, b2.RetailPrice, string(b2.Status)))
			},
			wantCount: 2,
		},
		{
			name:      "no batches",
			productID: "p2",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE product_id=$1`)).
					WithArgs("p2").
					WillReturnRows(sqlmock.NewRows(batchCols))
			},
			wantCount: 0,
		},
		{
			name:      "db error",
			productID: "p1",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE product_id=$1`)).
					WithArgs("p1").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			result, err := repo.ListByProduct(context.Background(), tc.productID)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  ListExpiring

func TestBatchRepository_ListExpiring(t *testing.T) {
	t.Parallel()
	b := newBatch("b1", "p1", time.Now().AddDate(0, 0, 10), 30, domain.BatchStatusExpiringSoon)

	tests := []struct {
		name      string
		daysAhead int
		setup     func(sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "has expiring batches",
			daysAhead: 30,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at <= $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(batchCols).AddRow(
						b.ID, b.ProductID, b.SeriesNumber, b.ExpiresAt, b.ReceivedAt,
						b.Quantity, b.Reserved, b.PurchasePrice, b.RetailPrice, string(b.Status),
					))
			},
			wantCount: 1,
		},
		{
			name:      "no expiring batches",
			daysAhead: 7,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at <= $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(batchCols))
			},
			wantCount: 0,
		},
		{
			name:      "db error",
			daysAhead: 30,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at <= $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			result, err := repo.ListExpiring(context.Background(), tc.daysAhead)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  ListExpired─

func TestBatchRepository_ListExpired(t *testing.T) {
	t.Parallel()
	expired := newBatch("b1", "p1", time.Now().AddDate(0, 0, -5), 20, domain.BatchStatusExpired)

	tests := []struct {
		name      string
		setup     func(sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "has expired batches",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at < $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(batchCols).AddRow(
						expired.ID, expired.ProductID, expired.SeriesNumber,
						expired.ExpiresAt, expired.ReceivedAt,
						expired.Quantity, expired.Reserved,
						expired.PurchasePrice, expired.RetailPrice, string(expired.Status),
					))
			},
			wantCount: 1,
		},
		{
			name: "no expired batches",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at < $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows(batchCols))
			},
			wantCount: 0,
		},
		{
			name: "db error",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(regexp.QuoteMeta(`WHERE expires_at < $1`)).
					WithArgs(sqlmock.AnyArg()).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			result, err := repo.ListExpired(context.Background())

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tc.wantCount)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  UpdateStatus

func TestBatchRepository_UpdateStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		batchID   string
		newStatus domain.BatchStatus
		setup     func(sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:      "to written_off",
			batchID:   "b1",
			newStatus: domain.BatchStatusWrittenOff,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE batches SET status`)).
					WithArgs("b1", string(domain.BatchStatusWrittenOff)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:      "to expired",
			batchID:   "b2",
			newStatus: domain.BatchStatusExpired,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE batches SET status`)).
					WithArgs("b2", string(domain.BatchStatusExpired)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:      "db error",
			batchID:   "b3",
			newStatus: domain.BatchStatusWrittenOff,
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE batches SET status`)).
					WithArgs("b3", string(domain.BatchStatusWrittenOff)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			err := repo.UpdateStatus(context.Background(), tc.batchID, tc.newStatus)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

//  Save

func TestBatchRepository_Save(t *testing.T) {
	t.Parallel()
	b := newBatch("b1", "p1", time.Now().AddDate(0, 6, 0), 80, domain.BatchStatusAvailable)

	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE batches SET quantity`)).
					WithArgs(b.ID, b.Quantity, b.Reserved, string(b.Status)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "db error",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectExec(regexp.QuoteMeta(`UPDATE batches SET quantity`)).
					WithArgs(b.ID, b.Quantity, b.Reserved, string(b.Status)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, mock := newBatchRepo(t)
			tc.setup(mock)

			err := repo.Save(context.Background(), b)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
