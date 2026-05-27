package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"mountain-race/camptocamp"
	"mountain-race/llm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRouter creates a Gin engine with all routes registered.
func newTestRouter() *gin.Engine {
	r := gin.New()
	Register(r)
	return r
}

// --- preferredLang ---

func TestPreferredLang(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"", "fr"},
		{"fr-FR,fr;q=0.9,en;q=0.8", "fr"},
		{"en-US,en;q=0.9", "en"},
		{"de", "de"},
		{"FR", "fr"},
	}
	for _, tt := range tests {
		got := preferredLang(tt.header)
		if got != tt.want {
			t.Errorf("preferredLang(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

// --- SearchRoutes ---

func TestSearchRoutes_InvalidJSON(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/routes/search", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSearchRoutes_C2CServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()
	camptocamp.SetBaseURL(srv.URL)
	defer camptocamp.ResetBaseURL()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{
		"location":   "Chamonix",
		"race_type":  "multipitch",
		"difficulty": "5c",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/routes/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSearchRoutes_Success(t *testing.T) {
	docs := []map[string]any{
		{
			"document_id":      float64(123),
			"rock_free_rating": "5c",
			"locales": []any{
				map[string]any{"lang": "fr", "title": "Test Route", "summary": "Une voie test."},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"documents": docs, "total": 1})
	}))
	defer srv.Close()
	camptocamp.SetBaseURL(srv.URL)
	defer camptocamp.ResetBaseURL()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{
		"location":   "Chamonix",
		"race_type":  "multipitch",
		"difficulty": "5c",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/routes/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "fr")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	routes, _ := result["routes"].([]any)
	if len(routes) == 0 {
		t.Error("expected at least one route in response")
	}
}

// --- GetRoute ---

func TestGetRoute_C2CError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	camptocamp.SetBaseURL(srv.URL)
	defer camptocamp.ResetBaseURL()

	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/routes/999", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetRoute_Success(t *testing.T) {
	doc := map[string]any{
		"document_id":      float64(123456),
		"climbing_rating":  "5c",
		"height_diff_up":   float64(650),
		"height_diff_down": float64(400),
		"locales": []any{
			map[string]any{
				"lang":        "fr",
				"title":       "Test Route",
				"description": "Une belle voie.",
			},
		},
		"geometry": map[string]any{
			"geom": `{"type":"Point","coordinates":[765071,5768286]}`,
		},
		"associations": map[string]any{
			"routes": []any{},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer srv.Close()
	camptocamp.SetBaseURL(srv.URL)
	defer camptocamp.ResetBaseURL()

	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/routes/123456", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["id"] != "123456" {
		t.Errorf("expected id=123456, got %v", result["id"])
	}
}

// --- GetWeather ---

func TestGetWeather_MissingParams(t *testing.T) {
	r := newTestRouter()

	tests := []struct {
		url    string
		wantStatus int
	}{
		{"/api/weather?lat=abc&lon=6.9&date=2026-06-01", http.StatusBadRequest},
		{"/api/weather?lat=45.9&lon=abc&date=2026-06-01", http.StatusBadRequest},
		{"/api/weather?lat=45.9&lon=6.9&date=notadate", http.StatusBadRequest},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tt.url, nil)
		r.ServeHTTP(w, req)
		if w.Code != tt.wantStatus {
			t.Errorf("GET %s: expected %d, got %d", tt.url, tt.wantStatus, w.Code)
		}
	}
}

// --- GetAvalancheImage ---

func TestGetAvalancheImage_InvalidMassifID(t *testing.T) {
	r := newTestRouter()

	tests := []struct {
		url        string
		wantStatus int
	}{
		{"/api/avalanche/image?massif_id=abc&type=montagne-risques", http.StatusBadRequest},
		{"/api/avalanche/image?massif_id=0&type=montagne-risques", http.StatusBadRequest},
		{"/api/avalanche/image?massif_id=-5&type=montagne-risques", http.StatusBadRequest},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tt.url, nil)
		r.ServeHTTP(w, req)
		if w.Code != tt.wantStatus {
			t.Errorf("GET %s: expected %d, got %d", tt.url, tt.wantStatus, w.Code)
		}
	}
}

func TestGetAvalancheImage_InvalidType(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/avalanche/image?massif_id=42&type=badtype", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- ExtractEquipment ---

func TestExtractEquipment_InvalidJSON(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/equipment/extract", bytes.NewBufferString("{bad json}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestExtractEquipment_EmptyGearText(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"gear_text": ""})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/equipment/extract", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	equip, _ := result["equipment"].([]any)
	if len(equip) != 0 {
		t.Errorf("expected empty equipment list, got %v", equip)
	}
}

func TestExtractEquipment_WhitespaceGearText(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"gear_text": "   "})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/equipment/extract", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for whitespace-only gear text, got %d", w.Code)
	}
}

func TestExtractEquipment_LLMSuccess(t *testing.T) {
	// Replace the equipExtract var with a stub
	orig := equipExtract
	equipExtract = func(ctx context.Context, gearText, lang string) ([]llm.EquipmentItem, error) {
		return []llm.EquipmentItem{
			{Name: "Corde 60m", Quantity: 1, Notes: "obligatoire"},
		}, nil
	}
	defer func() { equipExtract = orig }()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"gear_text": "Corde 60m"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/equipment/extract", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	equip, _ := result["equipment"].([]any)
	if len(equip) != 1 {
		t.Fatalf("expected 1 item, got %d", len(equip))
	}
}

func TestExtractEquipment_LLMError(t *testing.T) {
	orig := equipExtract
	equipExtract = func(ctx context.Context, gearText, lang string) ([]llm.EquipmentItem, error) {
		return nil, context.DeadlineExceeded
	}
	defer func() { equipExtract = orig }()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"gear_text": "some gear"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/equipment/extract", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- GetImage ---

func TestGetImage_MissingName(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/images?source=CampToCamp", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetImage_UnsupportedSource(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/images?source=unknown&name=foo.jpg", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetImage_CampToCamp_Success(t *testing.T) {
	imgData := []byte("FAKE_IMAGE")
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imgData)
	}))
	defer imgSrv.Close()

	// Redirect all outbound HTTP to our fake image server
	origTransport := http.DefaultTransport
	http.DefaultTransport = &apiHostRewriter{target: imgSrv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/images?source=CampToCamp&name=test.jpg", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// apiHostRewriter redirects all outbound HTTP to a test server.
type apiHostRewriter struct {
	target string
	base   http.RoundTripper
}

func (h *apiHostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	host := h.target
	if len(host) > 7 {
		host = host[7:] // strip "http://"
	}
	cloned.URL.Host = host
	return h.base.RoundTrip(cloned)
}
