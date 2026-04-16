package meteo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const tokenURL = "https://portail-api.meteofrance.fr/token"

type tokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

var cache tokenCache

// Token returns a valid Bearer token, refreshing from MeteoFrance if needed.
func Token() (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.token != "" && time.Now().Before(cache.expiresAt) {
		return cache.token, nil
	}

	user := os.Getenv("METEOFRANCE_USER")
	pass := os.Getenv("METEOFRANCE_PASS")
	if user == "" || pass == "" {
		return "", fmt.Errorf("METEOFRANCE_USER / METEOFRANCE_PASS not set")
	}

	body := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(user, pass)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("token parse: %w", err)
	}

	cache.token = result.AccessToken
	cache.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return cache.token, nil
}
