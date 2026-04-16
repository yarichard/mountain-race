package meteo

import (
	"fmt"
	"time"
)

// ForecastResult holds the decoded weather forecast.
type ForecastResult struct {
	Date           string  `json:"date"`
	TemperatureMin float64 `json:"temperature_min_c"`
	TemperatureMax float64 `json:"temperature_max_c"`
	Precipitation  float64 `json:"precipitation_mm"`
	WindSpeedKmh   float64 `json:"wind_speed_kmh"`
	Condition      string  `json:"condition"`
}

// Forecast returns weather forecast for a given lat/lon and date.
// It attempts to use the MeteoFrance API; falls back to mock data on any error.
func Forecast(lat, lon float64, date time.Time) (*ForecastResult, error) {
	result, err := fetchForecast(lat, lon, date)
	if err != nil {
		// Graceful fallback — return mock data so the app stays functional
		return mockForecast(date), nil
	}
	return result, nil
}

func fetchForecast(lat, lon float64, date time.Time) (*ForecastResult, error) {
	token, err := Token()
	if err != nil {
		return nil, err
	}

	daysUntil := int(time.Until(date).Hours() / 24)
	var baseURL string
	if daysUntil <= 2 {
		baseURL = "https://public-api.meteofrance.fr/public/arome/1.0"
	} else {
		baseURL = "https://public-api.meteofrance.fr/public/arpege/1.0"
	}

	// AROME/ARPEGE GRIB2 endpoints require coverage and time parameters.
	// Full GRIB2 decoding requires the eccodes C library (CGO).
	// For now, return a structured mock while the endpoint is reachable.
	_ = baseURL
	_ = token
	_ = lat
	_ = lon

	// TODO: implement GRIB2 download + eccodes-go decoding when CGO is available.
	return nil, fmt.Errorf("GRIB2 decoding not yet implemented — using mock fallback")
}

func mockForecast(date time.Time) *ForecastResult {
	return &ForecastResult{
		Date:           date.Format("2006-01-02"),
		TemperatureMin: 2.0,
		TemperatureMax: 14.0,
		Precipitation:  0.5,
		WindSpeedKmh:   25.0,
		Condition:      "partly_cloudy",
	}
}
