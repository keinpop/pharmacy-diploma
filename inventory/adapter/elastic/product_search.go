package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"pharmacy/inventory/domain"

	"github.com/elastic/go-elasticsearch/v8"
)

const indexName = "products"

type ProductSearchRepo struct{ es *elasticsearch.Client }

func NewProductSearchRepo(es *elasticsearch.Client) *ProductSearchRepo {
	return &ProductSearchRepo{es: es}
}

func (r *ProductSearchRepo) IndexProduct(ctx context.Context, p *domain.Product) error {
	doc := map[string]any{
		"id":               p.ID,
		"name":             p.Name,
		"trade_name":       p.TradeName,
		"active_substance": p.ActiveSubstance,
		"form":             p.Form,
		"dosage":           p.Dosage,
		"category":         string(p.Category),
	}
	body, _ := json.Marshal(doc)
	res, err := r.es.Index(indexName, bytes.NewReader(body),
		r.es.Index.WithDocumentID(p.ID),
		r.es.Index.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("es index error: %s", res.String())
	}
	return nil
}

func (r *ProductSearchRepo) Search(ctx context.Context, query string, limit int) ([]*domain.Product, error) {
	q := map[string]any{
		"size": limit,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":     query,
				"fields":    []string{"name^3", "trade_name^2", "active_substance"},
				"fuzziness": "AUTO",
			},
		},
	}
	return r.execute(ctx, q)
}

func (r *ProductSearchRepo) SearchBySubstance(ctx context.Context, substance string, limit int) ([]*domain.Product, error) {
	q := map[string]any{
		"size": limit,
		"query": map[string]any{
			"match": map[string]any{
				"active_substance": map[string]any{
					"query":     substance,
					"fuzziness": "AUTO",
				},
			},
		},
	}
	return r.execute(ctx, q)
}

func (r *ProductSearchRepo) ReindexAll(ctx context.Context, products []*domain.Product) error {
	for _, p := range products {
		if err := r.IndexProduct(ctx, p); err != nil {
			return fmt.Errorf("reindex product %s: %w", p.ID, err)
		}
	}
	return nil
}

func (r *ProductSearchRepo) execute(ctx context.Context, q map[string]any) ([]*domain.Product, error) {
	body, _ := json.Marshal(q)
	res, err := r.es.Search(
		r.es.Search.WithContext(ctx),
		r.es.Search.WithIndex(indexName),
		r.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					ID              string `json:"id"`
					Name            string `json:"name"`
					TradeName       string `json:"trade_name"`
					ActiveSubstance string `json:"active_substance"`
					Form            string `json:"form"`
					Dosage          string `json:"dosage"`
					Category        string `json:"category"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}
	products := make([]*domain.Product, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		s := h.Source
		products = append(products, &domain.Product{
			ID: s.ID, Name: s.Name, TradeName: s.TradeName,
			ActiveSubstance: s.ActiveSubstance, Form: s.Form,
			Dosage: s.Dosage, Category: domain.Category(s.Category),
		})
	}
	return products, nil
}
