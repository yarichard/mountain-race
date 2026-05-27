package meteo

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// massifGeoJSON builds a minimal GeoJSON FeatureCollection for massif list.
// The polygon is a 1°×1° box around the given centroid.
func massifGeoJSON(id int, name string, centLat, centLon float64) []byte {
	half := 0.5
	fc := map[string]any{
		"type": "FeatureCollection",
		"features": []any{
			map[string]any{
				"type": "Feature",
				"properties": map[string]any{
					"code":       float64(id),
					"title":      name,
					"lat_center": centLat,
					"lon_center": centLon,
				},
				"geometry": map[string]any{
					// MultiPolygon: one polygon, one ring. GeoJSON is [lon, lat].
					"coordinates": [][][][]float64{{{
						{centLon - half, centLat - half},
						{centLon + half, centLat - half},
						{centLon + half, centLat + half},
						{centLon - half, centLat + half},
						{centLon - half, centLat - half},
					}}},
				},
			},
		},
	}
	b, _ := json.Marshal(fc)
	return b
}

// braXMLResponse builds a minimal BRA XML for the given date and risk level.
func braXMLResponse(dateStr string, riskLevel int) []byte {
	type risque struct {
		XMLName    xml.Name `xml:"RISQUE"`
		Date       string   `xml:"DATE,attr"`
		RisqueMaxi int      `xml:"RISQUEMAXI,attr"`
	}
	type risques struct {
		XMLName xml.Name `xml:"RISQUES"`
		Items   []risque
	}
	type bra struct {
		XMLName xml.Name `xml:"BRA"`
		Risques risques
	}
	b := bra{Risques: risques{Items: []risque{{Date: dateStr + "T00:00:00", RisqueMaxi: riskLevel}}}}
	out, _ := xml.Marshal(b)
	return out
}

// avalancheTestServer builds a combined test server handling:
//
//	/token            → Bearer token JSON
//	/public/DPBRA/v1/liste-massifs → massifJSON
//	/public/DPBRA/v1/massif/BRA   → braXMLData
//	anything else                  → PNG placeholder
func avalancheTestServer(t *testing.T, massifJSON, braXMLData []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-token-123",
				"expires_in":   3600,
			})
		case "/public/DPBRA/v1/liste-massifs":
			w.Header().Set("Content-Type", "application/json")
			w.Write(massifJSON)
		case "/public/DPBRA/v1/massif/BRA":
			w.Header().Set("Content-Type", "application/xml")
			w.Write(braXMLData)
		default:
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("PNG_IMAGE_DATA"))
		}
	}))
}

// resetTokenCache clears the cached token so tests get fresh ones.
func resetTokenCache() {
	cache.mu.Lock()
	cache.token = ""
	cache.expiresAt = cache.expiresAt.AddDate(-10, 0, 0)
	cache.mu.Unlock()
}

// setOverrides installs URL overrides and returns a cleanup func.
func setOverrides(tokenURL, dpbraURL string) func() {
	tokenURLOverride = tokenURL
	dpbraBaseOverride = dpbraURL
	return func() {
		tokenURLOverride = ""
		dpbraBaseOverride = ""
	}
}

func TestAvalancheForecast_HappyPath(t *testing.T) {
	date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	dateStr := "2026-05-15"

	massifJSON := massifGeoJSON(42, "Belledonne", 45.2, 6.1)
	braXML := braXMLResponse(dateStr, 3)

	srv := avalancheTestServer(t, massifJSON, braXML)
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "user")
	t.Setenv("METEOFRANCE_PASS", "pass")

	// Point inside the Belledonne box (45.2±0.5, 6.1±0.5)
	result, err := AvalancheForecast(45.2, 6.1, date)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != 3 {
		t.Errorf("RiskLevel: got %d, want 3", result.RiskLevel)
	}
	if result.RiskLabel != "Marqué" {
		t.Errorf("RiskLabel: got %q, want Marqué", result.RiskLabel)
	}
	if result.MassifID != 42 {
		t.Errorf("MassifID: got %d, want 42", result.MassifID)
	}
	if result.MassifName != "Belledonne" {
		t.Errorf("MassifName: got %q, want Belledonne", result.MassifName)
	}
}

func TestAvalancheForecast_MissingCredentials(t *testing.T) {
	resetTokenCache()
	os.Unsetenv("METEOFRANCE_USER")
	os.Unsetenv("METEOFRANCE_PASS")

	_, err := AvalancheForecast(45.2, 6.1, time.Now())
	if err == nil {
		t.Fatal("expected error when credentials are missing")
	}
}

func TestAvalancheForecast_TokenEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", "")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	_, err := AvalancheForecast(45.2, 6.1, time.Now())
	if err == nil {
		t.Fatal("expected error for token endpoint failure")
	}
}

func TestAvalancheForecast_PointOutsideMassif(t *testing.T) {
	massifJSON := massifGeoJSON(42, "Belledonne", 45.2, 6.1)
	braXML := braXMLResponse("2026-01-01", 1)

	srv := avalancheTestServer(t, massifJSON, braXML)
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	// Point far away from the massif (50.0, 2.0 is outside [44.7-45.7, 5.6-6.6])
	_, err := AvalancheForecast(50.0, 2.0, time.Now())
	if err == nil {
		t.Fatal("expected error when point is not inside any massif")
	}
}

func TestProxyMassifImage_HappyPath(t *testing.T) {
	imageData := []byte("FAKE_PNG_DATA")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(imageData)
	}))
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	var buf bytes.Buffer
	ct, err := ProxyMassifImage(&buf, 42, "montagne-risques")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "image/png" {
		t.Errorf("content-type: got %q, want image/png", ct)
	}
	if !bytes.Equal(buf.Bytes(), imageData) {
		t.Errorf("image data mismatch: got %q, want %q", buf.Bytes(), imageData)
	}
}

func TestProxyMassifImage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	var buf bytes.Buffer
	_, err := ProxyMassifImage(&buf, 42, "montagne-risques")
	if err == nil {
		t.Fatal("expected error for 404 image response")
	}
}

func TestToken_CachedToken(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", callCount),
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL, "")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	tok1, err := Token()
	if err != nil {
		t.Fatalf("first Token() error: %v", err)
	}
	tok2, err := Token()
	if err != nil {
		t.Fatalf("second Token() error: %v", err)
	}
	if tok1 != tok2 {
		t.Errorf("expected same cached token, got %q then %q", tok1, tok2)
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

func TestAvalancheForecast_FallbackToFirstEntry(t *testing.T) {
	date := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)

	massifJSON := massifGeoJSON(42, "Belledonne", 45.2, 6.1)
	// BRA has a different date — code should fall back to first entry
	braXML := braXMLResponse("2026-05-20", 4)

	srv := avalancheTestServer(t, massifJSON, braXML)
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	result, err := AvalancheForecast(45.2, 6.1, date)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RiskLevel != 4 {
		t.Errorf("RiskLevel: got %d, want 4 (fallback to first entry)", result.RiskLevel)
	}
}

func TestToken_MissingEnvVars(t *testing.T) {
	resetTokenCache()
	os.Unsetenv("METEOFRANCE_USER")
	os.Unsetenv("METEOFRANCE_PASS")

	_, err := Token()
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func TestAvalancheForecast_InvalidMassifJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
			return
		}
		// Return invalid JSON for massifs
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	resetTokenCache()
	defer setOverrides(srv.URL+"/token", srv.URL+"/public/DPBRA/v1")()

	t.Setenv("METEOFRANCE_USER", "u")
	t.Setenv("METEOFRANCE_PASS", "p")

	_, err := AvalancheForecast(45.2, 6.1, time.Now())
	if err == nil {
		t.Fatal("expected error for invalid massif JSON")
	}
}
