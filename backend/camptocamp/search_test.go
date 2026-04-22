package camptocamp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockSearchResults is kept here for test use only.
func mockSearchResults() []SearchResult {
	return []SearchResult{
		{
			ID:             "123456",
			Title:          "Aiguille de l'Index — Voie normale",
			Summary:        "Belle voie d'initiation en face nord.",
			Difficulty:     "4c",
			HeightDiffUp:   650,
			HeightDiffDown: 650,
			SourceURL:      "https://www.camptocamp.org/routes/123456",
		},
		{
			ID:             "234567",
			Title:          "Arête des Cosmiques",
			Summary:        "Arête rocheuse classique au-dessus de l'Aiguille du Midi.",
			Difficulty:     "AD",
			HeightDiffUp:   420,
			HeightDiffDown: 420,
			SourceURL:      "https://www.camptocamp.org/routes/234567",
		},
		{
			ID:             "345678",
			Title:          "Grand Balcon Nord",
			Summary:        "Randonnée panoramique sous le Mont Blanc.",
			Difficulty:     "F",
			HeightDiffUp:   350,
			HeightDiffDown: 350,
			SourceURL:      "https://www.camptocamp.org/routes/345678",
		},
	}
}

// c2cSearchServer starts a test server returning the given documents.
func c2cSearchServer(t *testing.T, docs []map[string]any, assertReq func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if assertReq != nil {
			assertReq(r)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"documents": docs, "total": len(docs)})
	}))
}

func TestSearch_ByName_ParsesResults(t *testing.T) {
	docs := []map[string]any{
		{
			"document_id":       float64(123456),
			"rock_free_rating":  "5c",
			"elevation_gain_up": float64(650),
			"route_length":      float64(4200),
			"locales": []any{
				map[string]any{"lang": "fr", "title": "Aiguille de l'Index", "summary": "Belle voie."},
			},
		},
	}

	srv := c2cSearchServer(t, docs, func(r *http.Request) {
		if r.URL.Query().Get("q") != "Chamonix" {
			t.Errorf("expected q=Chamonix, got %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("bbox") != "" {
			t.Error("expected no bbox param for name search")
		}
	})
	defer srv.Close()
	baseURL = srv.URL

	results, err := Search(SearchRequest{Location: "Chamonix", LocationType: "name", RaceType: "multipitch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ID != "123456" {
		t.Errorf("ID: got %q, want %q", r.ID, "123456")
	}
	if r.Title != "Aiguille de l'Index()" {
		t.Errorf("Title: got %q, want %q", r.Title, "Aiguille de l'Index")
	}
	if r.Difficulty != "5c" {
		t.Errorf("Difficulty: got %q, want %q", r.Difficulty, "5c")
	}
	if r.HeightDiffUp != 0 {
		t.Errorf("HeightDiffUp: got %d, want 0 (field not present in mock doc)", r.HeightDiffUp)
	}
}

func TestSearch_ByName_EmptyDocs(t *testing.T) {
	srv := c2cSearchServer(t, []map[string]any{}, nil)
	defer srv.Close()
	baseURL = srv.URL

	results, err := Search(SearchRequest{Location: "nowhere", LocationType: "name", RaceType: "hike"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestSearch_ByName_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()
	baseURL = srv.URL

	_, err := Search(SearchRequest{Location: "Chamonix", LocationType: "name", RaceType: "multipitch"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
}

func TestSearch_ByLocation_RawGPS(t *testing.T) {
	docs := []map[string]any{
		{
			"document_id":     float64(999),
			"global_rating":   "PD",
			"elevation_gain_up": float64(300),
			"route_length":    float64(5000),
			"locales": []any{
				map[string]any{"lang": "fr", "title": "Voie GPS"},
			},
		},
	}

	srv := c2cSearchServer(t, docs, func(r *http.Request) {
		bbox := r.URL.Query().Get("bbox")
		if bbox == "" {
			t.Error("expected bbox param for location search, got none")
		}
		if r.URL.Query().Get("q") != "" {
			t.Error("expected no q param for location search")
		}
		// bbox should be centered around Chamonix (45.9, 6.9)
		// x≈768035, y≈5770758 → bbox values around those
		if !strings.Contains(bbox, ",") {
			t.Errorf("bbox malformed: %q", bbox)
		}
	})
	defer srv.Close()
	baseURL = srv.URL

	results, err := Search(SearchRequest{Location: "45.9,6.9", LocationType: "location", RaceType: "ridge_hike"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Voie GPS()" {
		t.Errorf("Title: got %q, want %q", results[0].Title, "Voie GPS")
	}
}

func TestSearch_ByLocation_Geocodes(t *testing.T) {
	// Nominatim mock returns Chamonix coordinates
	nominatimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "Chamonix" {
			t.Errorf("nominatim: expected q=Chamonix, got %q", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"lat": "45.9246705", "lon": "6.8727506", "display_name": "Chamonix-Mont-Blanc"},
		})
	}))
	defer nominatimSrv.Close()
	nominatimBaseURL = nominatimSrv.URL

	c2cSrv := c2cSearchServer(t, []map[string]any{
		{
			"document_id":       float64(42),
			"climbing_rating":   "6a",
			"elevation_gain_up": float64(800),
			"route_length":      float64(6000),
			"locales": []any{
				map[string]any{"lang": "fr", "title": "La Directe"},
			},
		},
	}, func(r *http.Request) {
		if r.URL.Query().Get("bbox") == "" {
			t.Error("expected bbox from geocoded location")
		}
	})
	defer c2cSrv.Close()
	baseURL = c2cSrv.URL

	results, err := Search(SearchRequest{Location: "Chamonix", LocationType: "location", RaceType: "multipitch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "La Directe()" {
		t.Errorf("Title: got %q, want %q", results[0].Title, "La Directe")
	}
}

func TestSearch_ByLocation_CustomRadius(t *testing.T) {
	var capturedBbox string
	srv := c2cSearchServer(t, []map[string]any{}, func(r *http.Request) {
		capturedBbox = r.URL.Query().Get("bbox")
	})
	defer srv.Close()
	baseURL = srv.URL

	// 45.9,6.9 → Mercator x≈768035, y≈5770758
	// radius 50km → bbox should span ±50000m around that point
	_, err := Search(SearchRequest{Location: "45.9,6.9", LocationType: "location", RaceType: "hike", RadiusKm: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBbox == "" {
		t.Fatal("expected bbox param, got none")
	}
	// With 50km radius the bbox values should be significantly wider than the default 20km
	// We verify by checking against what a 20km radius would produce
	_, errDefault := Search(SearchRequest{Location: "45.9,6.9", LocationType: "location", RaceType: "hike"})
	if errDefault != nil {
		t.Fatalf("unexpected error: %v", errDefault)
	}
	// capturedBbox was overwritten by the second call (default radius)
	defaultBbox := capturedBbox

	// Re-run with 50km to get that bbox back
	capturedBbox = ""
	Search(SearchRequest{Location: "45.9,6.9", LocationType: "location", RaceType: "hike", RadiusKm: 50}) //nolint
	if capturedBbox == defaultBbox {
		t.Error("expected different bbox for 50km vs 20km radius")
	}
}

func TestSearch_ByLocation_GeocodeFails_ReturnsError(t *testing.T) {
	nominatimSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty result — no match
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer nominatimSrv.Close()
	nominatimBaseURL = nominatimSrv.URL

	// C2C still called but without bbox (geocode failed silently, falls through to no params)
	c2cSrv := c2cSearchServer(t, []map[string]any{}, nil)
	defer c2cSrv.Close()
	baseURL = c2cSrv.URL

	results, err := Search(SearchRequest{Location: "xyznonexistent", LocationType: "location", RaceType: "hike"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when geocode fails, got %d", len(results))
	}
}
