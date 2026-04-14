package elastic_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	esadapter "pharmacy/inventory/adapter/elastic"
	"pharmacy/inventory/domain"
)

// esResponse формирует ответ Elasticsearch для набора продуктов.
func esResponse(products []map[string]any) []byte {
	hits := make([]map[string]any, 0, len(products))
	for _, p := range products {
		hits = append(hits, map[string]any{"_source": p})
	}
	body, _ := json.Marshal(map[string]any{
		"hits": map[string]any{"hits": hits},
	})
	return body
}

// newTestRepo создаёт репо с фейковым HTTP-сервером вместо реального ES.
func newTestRepo(t *testing.T, responseBody []byte, wantStatusCode int) *esadapter.ProductSearchRepo {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Читаем тело запроса (query), чтобы не блокировать соединение.
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(wantStatusCode)
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(srv.Close)

	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:            []string{srv.URL},
		DiscoverNodesOnStart: false,
	})
	require.NoError(t, err)
	return esadapter.NewProductSearchRepo(client)
}

//  IndexProduct

func TestIndexProduct(t *testing.T) {
	okBody, _ := json.Marshal(map[string]any{"result": "created"})

	tests := []struct {
		name       string
		product    *domain.Product
		statusCode int
		wantErr    bool
	}{
		{
			name:       "успешная индексация",
			product:    &domain.Product{ID: "p1", Name: "Аспирин", ActiveSubstance: "аск", Category: domain.CategoryOTC},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "ES вернул ошибку 500",
			product:    &domain.Product{ID: "p2", Name: "Ибупрофен"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTestRepo(t, okBody, tc.statusCode)
			err := repo.IndexProduct(context.Background(), tc.product)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//  Search

func TestSearch(t *testing.T) {
	aspirin := map[string]any{
		"id": "p1", "name": "Аспирин", "trade_name": "Aspirin Cardio",
		"active_substance": "аск", "form": "таблетка", "dosage": "100мг", "category": "otc",
	}
	ibuprofen := map[string]any{
		"id": "p2", "name": "Ибупрофен", "trade_name": "Нурофен",
		"active_substance": "ибупрофен", "form": "таблетка", "dosage": "200мг", "category": "otc",
	}

	tests := []struct {
		name        string
		query       string
		limit       int
		esProducts  []map[string]any
		wantCount   int
		wantFirstID string
	}{
		{
			name:        "один результат",
			query:       "аспирин",
			limit:       5,
			esProducts:  []map[string]any{aspirin},
			wantCount:   1,
			wantFirstID: "p1",
		},
		{
			name:        "несколько результатов",
			query:       "таблетка",
			limit:       10,
			esProducts:  []map[string]any{aspirin, ibuprofen},
			wantCount:   2,
			wantFirstID: "p1",
		},
		{
			name:       "пустой результат",
			query:      "несуществующий препарат",
			limit:      5,
			esProducts: []map[string]any{},
			wantCount:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTestRepo(t, esResponse(tc.esProducts), http.StatusOK)
			products, err := repo.Search(context.Background(), tc.query, tc.limit)
			require.NoError(t, err)
			assert.Len(t, products, tc.wantCount)
			if tc.wantCount > 0 {
				assert.Equal(t, tc.wantFirstID, products[0].ID)
			}
		})
	}
}

//  SearchBySubstance

func TestSearchBySubstance(t *testing.T) {
	aspirin := map[string]any{
		"id": "p1", "name": "Аспирин", "active_substance": "аск", "category": "otc",
	}
	aspirin2 := map[string]any{
		"id": "p3", "name": "Тромбо АСС", "active_substance": "аск", "category": "otc",
	}

	tests := []struct {
		name       string
		substance  string
		limit      int
		esProducts []map[string]any
		wantCount  int
		wantIDs    []string
	}{
		{
			name:       "аналоги по веществу",
			substance:  "аск",
			limit:      5,
			esProducts: []map[string]any{aspirin, aspirin2},
			wantCount:  2,
			wantIDs:    []string{"p1", "p3"},
		},
		{
			name:       "вещество не найдено",
			substance:  "несуществующее",
			limit:      5,
			esProducts: []map[string]any{},
			wantCount:  0,
		},
		{
			name:       "лимит применяется на стороне ES",
			substance:  "аск",
			limit:      1,
			esProducts: []map[string]any{aspirin}, // ES вернул 1 из-за limit
			wantCount:  1,
			wantIDs:    []string{"p1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTestRepo(t, esResponse(tc.esProducts), http.StatusOK)
			products, err := repo.SearchBySubstance(context.Background(), tc.substance, tc.limit)
			require.NoError(t, err)
			assert.Len(t, products, tc.wantCount)
			for i, id := range tc.wantIDs {
				assert.Equal(t, id, products[i].ID)
			}
		})
	}
}
