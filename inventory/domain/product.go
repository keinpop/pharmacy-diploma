package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Category string

const (
	CategoryPrescription Category = "prescription" // по рецепту
	CategoryOTC          Category = "otc"          // безрецептурные препараты
)

var (
	ErrEmptyProductName     = errors.New("product name is required")
	ErrEmptyActiveSubstance = errors.New("active substance is required")
	ErrInvalidCategory      = errors.New("invalid category")
	ErrEmptySearchQuery     = errors.New("search query is required")
	ErrInsufficientStock    = errors.New("insufficient total stock")
)

type Product struct {
	ID                string
	Name              string   // торговое наименование в системе
	TradeName         string   // коммерческое название
	ActiveSubstance   string   // МНН (международное непатентованное наименование)
	Form              string   // таблетки, капсулы, раствор...
	Dosage            string   // 100мг, 500мг/мл...
	Category          Category // "otc" или "prescription"
	StorageConditions string
	Unit              string // единица учёта
	TherapeuticGroup  string // терапевтическая группа
	ReorderPoint      int    // порог для уведомления о низком остатке
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewProduct(
	name, tradeName, activeSubstance, form, dosage string,
	category Category,
	storageConditions, unit string,
	reorderPoint int, therapeuticGroup string,
) (*Product, error) {
	if name == "" {
		return nil, ErrEmptyProductName
	}
	if activeSubstance == "" {
		return nil, ErrEmptyActiveSubstance
	}
	if category != CategoryPrescription && category != CategoryOTC {
		return nil, ErrInvalidCategory
	}
	now := time.Now()
	return &Product{
		ID:                uuid.NewString(),
		Name:              name,
		TradeName:         tradeName,
		ActiveSubstance:   activeSubstance,
		Form:              form,
		Dosage:            dosage,
		Category:          category,
		StorageConditions: storageConditions,
		Unit:              unit,
		TherapeuticGroup:  therapeuticGroup,
		ReorderPoint:      reorderPoint,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}
