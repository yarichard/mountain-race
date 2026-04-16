package camptocamp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var baseURL = "https://api.camptocamp.org"

var httpClient = &http.Client{Timeout: 15 * time.Second}

func get(path string) (map[string]any, error) {
	resp, err := httpClient.Get(baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("camptocamp GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("camptocamp JSON decode: %w", err)
	}
	return result, nil
}

// stringField safely reads a string from a map.
func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// intField safely reads an int from a map (JSON numbers are float64).
func intField(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

// floatField safely reads a float64 from a map.
func floatField(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return 0
}

// localizedString extracts the French or first available localized string from a C2C locale map.
func localizedString(m map[string]any, key string) string {
	raw, ok := m[key]
	if !ok || raw == nil {
		return ""
	}
	loc, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	for _, lang := range []string{"fr", "en", "de", "it", "es"} {
		if s, ok := loc[lang].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
