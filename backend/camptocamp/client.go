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
	route := baseURL + path
	resp, err := httpClient.Get(route)
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

// pickLocale selects the best localized field value from C2C's locales array.
// It tries the preferred language first, then "fr", then "en", then the first available.
func pickLocale(locales []any, lang, field string) string {
	prefs := []string{lang, "fr", "en"}
	seen := map[string]bool{}
	for _, p := range prefs {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		for _, l := range locales {
			lm, ok := l.(map[string]any)
			if !ok {
				continue
			}
			if lm["lang"] == p {
				if v, ok := lm[field].(string); ok && v != "" {
					return v
				}
			}
		}
	}
	// Fallback: first non-empty regardless of language.
	for _, l := range locales {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := lm[field].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// localesField extracts the locales array from a C2C document map.
func localesField(m map[string]any) []any {
	locs, _ := m["locales"].([]any)
	return locs
}
