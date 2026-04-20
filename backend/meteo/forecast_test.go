package meteo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockOpenMeteoResponse builds a minimal Open-Meteo JSON response for the given date
// with constant temperature and wind values across all 24 hours.
func mockOpenMeteoResponse(dateStr string, tempC, windKmh, precipPerHour float64) []byte {
	times := make([]string, 24)
	temps := make([]float64, 24)
	winds := make([]float64, 24)
	precips := make([]float64, 24)
	for h := 0; h < 24; h++ {
		times[h] = dateStr + "T" + twoDigit(h) + ":00"
		temps[h] = tempC
		winds[h] = windKmh
		precips[h] = precipPerHour
	}
	resp := map[string]interface{}{
		"latitude":  45.9337,
		"longitude": 5.4905,
		"timezone":  "UTC",
		"hourly": map[string]interface{}{
			"time":            times,
			"temperature_100m": temps,
			"wind_speed_10m":  winds,
			"precipitation":   precips,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func twoDigit(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestFetchWeather_MockServer(t *testing.T) {
	date := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	dateStr := "2026-04-21"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mockOpenMeteoResponse(dateStr, 10.0, 36.0, 0.0))
	}))
	defer srv.Close()

	origBase := openMeteoBase
	openMeteoBase = srv.URL
	defer func() { openMeteoBase = origBase }()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	forecast, hourly, err := FetchWeather(45.9337, 5.4905, date)
	if err != nil {
		t.Fatalf("FetchWeather error: %v", err)
	}

	if forecast.Date != dateStr {
		t.Errorf("date: got %s, want %s", forecast.Date, dateStr)
	}
	if forecast.TemperatureMin != 10.0 || forecast.TemperatureMax != 10.0 {
		t.Errorf("temp min/max: got %v/%v, want 10/10", forecast.TemperatureMin, forecast.TemperatureMax)
	}
	if forecast.Precipitation != 0.0 {
		t.Errorf("precip: got %v, want 0", forecast.Precipitation)
	}
	if forecast.WindSpeedKmh != 36.0 {
		t.Errorf("wind: got %v, want 36", forecast.WindSpeedKmh)
	}
	if forecast.Condition != "sunny" {
		t.Errorf("condition: got %s, want sunny", forecast.Condition)
	}

	if len(hourly) != 24 {
		t.Fatalf("expected 24 hourly points, got %d", len(hourly))
	}
	for _, p := range hourly {
		if p.TemperatureC != 10.0 {
			t.Errorf("hour %d: expected temp 10.0°C, got %v", p.Hour, p.TemperatureC)
		}
		if p.WindSpeedKmh != 36.0 {
			t.Errorf("hour %d: expected wind 36 km/h, got %v", p.Hour, p.WindSpeedKmh)
		}
	}
}

func TestFetchWeather_Rain(t *testing.T) {
	date := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	dateStr := "2026-04-21"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 0.3mm/hour × 24h = 7.2mm total → rain condition, tempC=10 > 2
		w.Write(mockOpenMeteoResponse(dateStr, 10.0, 20.0, 0.3))
	}))
	defer srv.Close()

	origBase := openMeteoBase
	openMeteoBase = srv.URL
	defer func() { openMeteoBase = origBase }()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	forecast, _, err := FetchWeather(45.9337, 5.4905, date)
	if err != nil {
		t.Fatalf("FetchWeather error: %v", err)
	}
	if forecast.Condition != "rain" {
		t.Errorf("condition: got %s, want rain", forecast.Condition)
	}
}

func TestFetchWeather_Snow(t *testing.T) {
	date := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	dateStr := "2026-04-21"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 0.3mm/hour × 24h = 7.2mm total + temp -5°C → snow
		w.Write(mockOpenMeteoResponse(dateStr, -5.0, 20.0, 0.3))
	}))
	defer srv.Close()

	origBase := openMeteoBase
	openMeteoBase = srv.URL
	defer func() { openMeteoBase = origBase }()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	forecast, _, err := FetchWeather(45.9337, 5.4905, date)
	if err != nil {
		t.Fatalf("FetchWeather error: %v", err)
	}
	if forecast.Condition != "snow" {
		t.Errorf("condition: got %s, want snow", forecast.Condition)
	}
}

func TestFetchWeather_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	origBase := openMeteoBase
	openMeteoBase = srv.URL
	defer func() { openMeteoBase = origBase }()

	origClient := httpClient
	httpClient = srv.Client()
	defer func() { httpClient = origClient }()

	_, _, err := FetchWeather(45.9337, 5.4905, time.Now())
	if err == nil {
		t.Fatal("expected error on HTTP 400")
	}
}

// --- deriveCondition ---

func TestDeriveCondition(t *testing.T) {
	cases := []struct {
		temp, precip, wind float64
		want               string
	}{
		{-5, 10, 10, "snow"},
		{10, 10, 10, "rain"},
		{10, 2, 10, "partly_cloudy"},
		{10, 0, 0, "sunny"},
		{10, 0, 80, "partly_cloudy"},
	}
	for _, c := range cases {
		got := deriveCondition(c.temp, c.precip, c.wind)
		if got != c.want {
			t.Errorf("deriveCondition(%v,%v,%v) = %q, want %q", c.temp, c.precip, c.wind, got, c.want)
		}
	}
}
