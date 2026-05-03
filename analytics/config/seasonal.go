package config

// SeasonalCoefficients maps therapeutic_group -> [winter, spring, summer, autumn] coefficients.
var SeasonalCoefficients = map[string][4]float64{
	"cold_flu":         {1.5, 0.7, 0.4, 1.3},
	"antihistamine":    {0.5, 1.5, 1.3, 0.6},
	"painkiller":       {1.0, 1.0, 1.0, 1.0},
	"dermatology":      {0.7, 1.0, 1.4, 0.8},
	"gastrointestinal": {0.9, 1.0, 1.3, 0.9},
	"vitamins":         {1.3, 1.1, 0.7, 1.2},
	"cardiovascular":   {1.1, 1.0, 1.0, 1.1},
}

// GetSeasonalCoefficient returns the seasonal coefficient for a given therapeutic group and month (1-12).
// Seasons: Dec/Jan/Feb = winter (idx 0), Mar/Apr/May = spring (idx 1),
// Jun/Jul/Aug = summer (idx 2), Sep/Oct/Nov = autumn (idx 3).
func GetSeasonalCoefficient(group string, month int) float64 {
	coeffs, ok := SeasonalCoefficients[group]
	if !ok {
		return 1.0
	}
	var seasonIdx int
	switch month {
	case 12, 1, 2:
		seasonIdx = 0 // winter
	case 3, 4, 5:
		seasonIdx = 1 // spring
	case 6, 7, 8:
		seasonIdx = 2 // summer
	case 9, 10, 11:
		seasonIdx = 3 // autumn
	default:
		return 1.0
	}
	return coeffs[seasonIdx]
}
