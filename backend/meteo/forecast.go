package meteo

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{}

// HourlyPoint holds forecast data for one time slot.
type HourlyPoint struct {
	Hour         int     `json:"hour"`
	TemperatureC float64 `json:"temperature_c"`
	WindSpeedKmh float64 `json:"wind_speed_kmh"`
}

// ForecastResult holds the decoded weather forecast.
type ForecastResult struct {
	Date           string  `json:"date"`
	TemperatureMin float64 `json:"temperature_min_c"`
	TemperatureMax float64 `json:"temperature_max_c"`
	Precipitation  float64 `json:"precipitation_mm"`
	WindSpeedKmh   float64 `json:"wind_speed_kmh"`
	Condition      string  `json:"condition"`
}

type openMeteoResponse struct {
	Hourly struct {
		Time           []string  `json:"time"`
		Temperature100 []float64 `json:"temperature_100m"`
		Temperature120 []float64 `json:"temperature_120m"`
		WindSpeed      []float64 `json:"wind_speed_10m"`
		Precipitation  []float64 `json:"precipitation"`
	} `json:"hourly"`
}

var openMeteoBase = "https://api.open-meteo.com/v1/forecast"

// Forecast returns the daily weather summary for a location and date.
func Forecast(lat, lon float64, date time.Time) (*ForecastResult, error) {
	result, _, err := FetchWeather(lat, lon, date)
	return result, err
}

// FetchWeather calls Open-Meteo and returns the daily summary plus hourly points.
// For dates within 4 days it uses the meteofrance_seamless model (temperature_100m);
// beyond 4 days it uses the basic API without a model override (temperature_120m).
func FetchWeather(lat, lon float64, date time.Time) (*ForecastResult, []HourlyPoint, error) {
	dateStr := date.Format("2006-01-02")

	today := time.Now().UTC().Truncate(24 * time.Hour)
	raceDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	beyond4Days := raceDay.Sub(today) > 4*24*time.Hour

	var tempParam, modelParam string
	if beyond4Days {
		tempParam = "temperature_120m"
		modelParam = ""
	} else {
		tempParam = "temperature_100m"
		modelParam = "&models=meteofrance_seamless"
	}

	url := fmt.Sprintf(
		"%s?latitude=%.6f&longitude=%.6f&hourly=%s,wind_speed_10m,precipitation%s&timezone=UTC&start_date=%s&end_date=%s",
		openMeteoBase, lat, lon, tempParam, modelParam, dateStr, dateStr,
	)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("open-meteo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, nil, fmt.Errorf("open-meteo HTTP %d: %s", resp.StatusCode, body)
	}

	var omr openMeteoResponse
	if err := json.NewDecoder(resp.Body).Decode(&omr); err != nil {
		return nil, nil, fmt.Errorf("open-meteo decode: %w", err)
	}

	n := len(omr.Hourly.Time)
	if n == 0 {
		return nil, nil, fmt.Errorf("open-meteo: no hourly data for %s", dateStr)
	}

	tempMin := math.MaxFloat64
	tempMax := -math.MaxFloat64
	precipSum := 0.0
	windMax := 0.0
	var hourly []HourlyPoint

	for i := range n {
		t := omr.Hourly.Time[i]
		if !strings.HasPrefix(t, dateStr) {
			continue
		}

		hour := 0
		if len(t) >= 13 {
			hour, _ = strconv.Atoi(t[11:13])
		}

		temp, wind, precip := 0.0, 0.0, 0.0
		temps := omr.Hourly.Temperature100
		if beyond4Days {
			temps = omr.Hourly.Temperature120
		}
		if i < len(temps) {
			temp = temps[i]
		}
		if i < len(omr.Hourly.WindSpeed) {
			wind = omr.Hourly.WindSpeed[i]
		}
		if i < len(omr.Hourly.Precipitation) {
			precip = omr.Hourly.Precipitation[i]
		}

		if temp < tempMin {
			tempMin = temp
		}
		if temp > tempMax {
			tempMax = temp
		}
		precipSum += precip
		if wind > windMax {
			windMax = wind
		}

		hourly = append(hourly, HourlyPoint{
			Hour:         hour,
			TemperatureC: math.Round(temp*10) / 10,
			WindSpeedKmh: math.Round(wind),
		})
	}

	if tempMin == math.MaxFloat64 {
		return nil, nil, fmt.Errorf("open-meteo: no data for date %s", dateStr)
	}

	forecast := &ForecastResult{
		Date:           dateStr,
		TemperatureMin: math.Round(tempMin*10) / 10,
		TemperatureMax: math.Round(tempMax*10) / 10,
		Precipitation:  math.Round(precipSum*10) / 10,
		WindSpeedKmh:   math.Round(windMax),
		Condition:      deriveCondition(tempMax, precipSum, windMax),
	}

	return forecast, hourly, nil
}

func deriveCondition(tempC, precipMm, windKmh float64) string {
	switch {
	case precipMm > 5.0 && tempC <= 2.0:
		return "snow"
	case precipMm > 5.0:
		return "rain"
	case precipMm > 1.0 || windKmh > 40:
		return "partly_cloudy"
	case windKmh > 70:
		return "storm"
	default:
		return "sunny"
	}
}
