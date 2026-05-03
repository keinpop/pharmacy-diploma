package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"pharmacy/analytics/config"
)

// — tests: GetSeasonalCoefficient for known groups —

func TestGetSeasonalCoefficient_KnownGroups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		group    string
		month    int
		expected float64
	}{
		// cold_flu: {1.5, 0.7, 0.4, 1.3}
		{name: "cold_flu winter", group: "cold_flu", month: 1, expected: 1.5},
		{name: "cold_flu spring", group: "cold_flu", month: 4, expected: 0.7},
		{name: "cold_flu summer", group: "cold_flu", month: 7, expected: 0.4},
		{name: "cold_flu autumn", group: "cold_flu", month: 10, expected: 1.3},

		// antihistamine: {0.5, 1.5, 1.3, 0.6}
		{name: "antihistamine winter", group: "antihistamine", month: 2, expected: 0.5},
		{name: "antihistamine spring", group: "antihistamine", month: 5, expected: 1.5},
		{name: "antihistamine summer", group: "antihistamine", month: 8, expected: 1.3},
		{name: "antihistamine autumn", group: "antihistamine", month: 11, expected: 0.6},

		// painkiller: {1.0, 1.0, 1.0, 1.0}
		{name: "painkiller winter", group: "painkiller", month: 12, expected: 1.0},
		{name: "painkiller spring", group: "painkiller", month: 3, expected: 1.0},
		{name: "painkiller summer", group: "painkiller", month: 6, expected: 1.0},
		{name: "painkiller autumn", group: "painkiller", month: 9, expected: 1.0},

		// dermatology: {0.7, 1.0, 1.4, 0.8}
		{name: "dermatology winter", group: "dermatology", month: 1, expected: 0.7},
		{name: "dermatology spring", group: "dermatology", month: 4, expected: 1.0},
		{name: "dermatology summer", group: "dermatology", month: 7, expected: 1.4},
		{name: "dermatology autumn", group: "dermatology", month: 10, expected: 0.8},

		// gastrointestinal: {0.9, 1.0, 1.3, 0.9}
		{name: "gastrointestinal winter", group: "gastrointestinal", month: 2, expected: 0.9},
		{name: "gastrointestinal spring", group: "gastrointestinal", month: 5, expected: 1.0},
		{name: "gastrointestinal summer", group: "gastrointestinal", month: 8, expected: 1.3},
		{name: "gastrointestinal autumn", group: "gastrointestinal", month: 11, expected: 0.9},

		// vitamins: {1.3, 1.1, 0.7, 1.2}
		{name: "vitamins winter", group: "vitamins", month: 12, expected: 1.3},
		{name: "vitamins spring", group: "vitamins", month: 3, expected: 1.1},
		{name: "vitamins summer", group: "vitamins", month: 6, expected: 0.7},
		{name: "vitamins autumn", group: "vitamins", month: 9, expected: 1.2},

		// cardiovascular: {1.1, 1.0, 1.0, 1.1}
		{name: "cardiovascular winter", group: "cardiovascular", month: 1, expected: 1.1},
		{name: "cardiovascular spring", group: "cardiovascular", month: 4, expected: 1.0},
		{name: "cardiovascular summer", group: "cardiovascular", month: 7, expected: 1.0},
		{name: "cardiovascular autumn", group: "cardiovascular", month: 10, expected: 1.1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.GetSeasonalCoefficient(tc.group, tc.month)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// — tests: default coefficient for unknown group —

func TestGetSeasonalCoefficient_UnknownGroup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		group string
		month int
	}{
		{name: "empty group string winter", group: "", month: 1},
		{name: "empty group string summer", group: "", month: 7},
		{name: "unknown group winter", group: "antibiotics", month: 1},
		{name: "unknown group spring", group: "antibiotics", month: 4},
		{name: "unknown group summer", group: "antibiotics", month: 7},
		{name: "unknown group autumn", group: "antibiotics", month: 10},
		{name: "random unknown group", group: "nuclear_medicine", month: 6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.GetSeasonalCoefficient(tc.group, tc.month)
			assert.Equal(t, 1.0, got, "unknown group should return default coefficient 1.0")
		})
	}
}

// — tests: all seasons for every month —

func TestGetSeasonalCoefficient_AllMonths(t *testing.T) {
	t.Parallel()
	// Verify every calendar month maps to the correct season for cold_flu.
	// cold_flu: winter=1.5, spring=0.7, summer=0.4, autumn=1.3
	tests := []struct {
		month    int
		season   string
		expected float64
	}{
		{month: 1, season: "winter", expected: 1.5},
		{month: 2, season: "winter", expected: 1.5},
		{month: 3, season: "spring", expected: 0.7},
		{month: 4, season: "spring", expected: 0.7},
		{month: 5, season: "spring", expected: 0.7},
		{month: 6, season: "summer", expected: 0.4},
		{month: 7, season: "summer", expected: 0.4},
		{month: 8, season: "summer", expected: 0.4},
		{month: 9, season: "autumn", expected: 1.3},
		{month: 10, season: "autumn", expected: 1.3},
		{month: 11, season: "autumn", expected: 1.3},
		{month: 12, season: "winter", expected: 1.5},
	}

	for _, tc := range tests {
		t.Run("cold_flu_month_"+string(rune('0'+tc.month)), func(t *testing.T) {
			t.Parallel()
			got := config.GetSeasonalCoefficient("cold_flu", tc.month)
			assert.Equal(t, tc.expected, got,
				"month %d should map to %s with coefficient %.1f", tc.month, tc.season, tc.expected)
		})
	}
}

// — tests: season boundary correctness —

func TestGetSeasonalCoefficient_SeasonBoundaries(t *testing.T) {
	t.Parallel()
	// Verify boundary months transition correctly between seasons.
	// Using antihistamine: winter=0.5, spring=1.5, summer=1.3, autumn=0.6
	group := "antihistamine"
	tests := []struct {
		name     string
		month    int
		expected float64
	}{
		// Winter boundaries
		{name: "last winter month (Feb)", month: 2, expected: 0.5},
		{name: "first spring month (Mar)", month: 3, expected: 1.5},
		// Spring boundaries
		{name: "last spring month (May)", month: 5, expected: 1.5},
		{name: "first summer month (Jun)", month: 6, expected: 1.3},
		// Summer boundaries
		{name: "last summer month (Aug)", month: 8, expected: 1.3},
		{name: "first autumn month (Sep)", month: 9, expected: 0.6},
		// Autumn boundaries
		{name: "last autumn month (Nov)", month: 11, expected: 0.6},
		{name: "first winter month (Dec)", month: 12, expected: 0.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := config.GetSeasonalCoefficient(group, tc.month)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// — tests: seasonal coefficients map integrity —

func TestSeasonalCoefficients_MapIntegrity(t *testing.T) {
	t.Parallel()
	expectedGroups := []string{
		"cold_flu",
		"antihistamine",
		"painkiller",
		"dermatology",
		"gastrointestinal",
		"vitamins",
		"cardiovascular",
	}

	for _, group := range expectedGroups {
		t.Run("group_"+group+"_has_four_coefficients", func(t *testing.T) {
			t.Parallel()
			coeffs, ok := config.SeasonalCoefficients[group]
			assert.True(t, ok, "group %q should be present in SeasonalCoefficients map", group)
			assert.Len(t, coeffs, 4, "group %q should have exactly 4 seasonal coefficients", group)
			for i, c := range coeffs {
				assert.Greater(t, c, 0.0, "coefficient [%d] for %q must be positive", i, group)
			}
		})
	}
}
