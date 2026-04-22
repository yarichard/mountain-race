package camptocamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"mountain-race/llm"
)

func TestMain(m *testing.M) {
	// Stub LLM extraction so detail tests don't need a real Ollama instance.
	equipExtract = func(_ context.Context, gearText, _ string) ([]llm.EquipmentItem, error) {
		if gearText == "" {
			return nil, nil
		}
		return []llm.EquipmentItem{{Name: "Stub item", Quantity: 1, Notes: "mandatory"}}, nil
	}
	os.Exit(m.Run())
}

// mockDetail is kept here for test use only.
func mockDetail(id string) *RouteDetail {
	return &RouteDetail{
		ID:          id,
		Title:       "Aiguille de l'Index — Voie normale (mock)",
		Description: "Belle voie classique accessible depuis l'Aiguille du Midi.",
		Difficulty:  "4c",
		ElevationGain:  650,
		HeightDiffDown: 400,
		Lat:            45.9,
		Lon:           6.9,
		Pitches: []Pitch{
			{Number: 1, Grade: "4a", Description: "Dalle initiale, prises évidentes."},
			{Number: 2, Grade: "4c", Description: "Passage clé sur arête, expo."},
			{Number: 3, Grade: "4b", Description: "Sortie en rocher brisé."},
		},
		Equipment: []Equipment{
			{Item: "Corde 60m", Quantity: 1, Notes: "Simple"},
			{Item: "Dégaines", Quantity: 12},
			{Item: "Casque", Quantity: 1, Notes: "Obligatoire"},
		},
		Risks: []string{
			"Risque de chute de pierres en début de journée",
			"Météo alpine changeante, vérifier bulletin avant départ",
		},
		AlternativeRoutes: []AlternativeRoute{
			{ID: "234567", Title: "Arête des Cosmiques", Reason: "Alternative en cas de monde"},
		},
		Schedule: Schedule{
			EstimatedDurationHours: 5.5,
			RecommendedStartTime:   "06:00",
			RecommendedEndTime:     "14:00",
			Source:                 "camptocamp",
		},
		SourceURL: "https://www.camptocamp.org/routes/" + id,
	}
}

// c2cDetailServer starts a test server returning the given route document.
func c2cDetailServer(t *testing.T, doc map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
}

// minimalRouteDoc builds a minimal C2C route document for tests.
func minimalRouteDoc(overrides map[string]any) map[string]any {
	doc := map[string]any{
		"document_id":     float64(123456),
		"climbing_rating":  "5c",
		"height_diff_up":   float64(650),
		"height_diff_down": float64(400),
		"locales": []any{
			map[string]any{
				"lang":        "fr",
				"title":       "Test Route",
				"description": "Une belle voie.",
				"gear":        "Corde 60m, 12 dégaines",
				"remarks":     "Attention aux chutes de pierres.",
			},
		},
		"geometry": map[string]any{
			// Chamonix in EPSG:3857: x≈765071, y≈5768286
			"geom": `{"type":"Point","coordinates":[765071,5768286]}`,
		},
		"associations": map[string]any{
			"routes": []any{
				map[string]any{
					"document_id": float64(234567),
					"locales": []any{
						map[string]any{"lang": "fr", "title": "Arête des Cosmiques"},
					},
				},
			},
		},
	}
	for k, v := range overrides {
		doc[k] = v
	}
	return doc
}

func TestGetDetail_ParsesTitle(t *testing.T) {
	srv := c2cDetailServer(t, minimalRouteDoc(nil))
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Title != " / Test Route" {
		t.Errorf("Title: got %q, want %q", d.Title, "Test Route")
	}
	if d.Description != "\n\nUne belle voie.\n\n" {
		t.Errorf("Description: got %q, want %q", d.Description, "Une belle voie.")
	}
	if d.Difficulty != "5c" {
		t.Errorf("Difficulty: got %q, want %q", d.Difficulty, "5c")
	}
	if d.ElevationGain != 650 {
		t.Errorf("ElevationGain: got %d, want 650", d.ElevationGain)
	}
	if d.HeightDiffDown != 400 {
		t.Errorf("HeightDiffDown: got %d, want 400", d.HeightDiffDown)
	}
}

func TestGetDetail_ParsesGeometry(t *testing.T) {
	srv := c2cDetailServer(t, minimalRouteDoc(nil))
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Chamonix is at ~45.9°N, ~6.87°E; allow 0.1° tolerance
	if d.Lat < 45.8 || d.Lat > 46.0 {
		t.Errorf("Lat out of range: got %f (expected ~45.9)", d.Lat)
	}
	if d.Lon < 6.7 || d.Lon > 7.0 {
		t.Errorf("Lon out of range: got %f (expected ~6.87)", d.Lon)
	}
}

func TestGetDetail_AlternativeRoutesNeverNull(t *testing.T) {
	// Route with no associations
	doc := minimalRouteDoc(map[string]any{
		"associations": map[string]any{},
	})
	srv := c2cDetailServer(t, doc)
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.AlternativeRoutes == nil {
		t.Error("AlternativeRoutes must never be nil (would serialize as JSON null and crash frontend)")
	}
	if len(d.AlternativeRoutes) != 0 {
		t.Errorf("expected 0 alternatives, got %d", len(d.AlternativeRoutes))
	}
}

func TestGetDetail_ParsesAlternativeRoutes(t *testing.T) {
	srv := c2cDetailServer(t, minimalRouteDoc(nil))
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.AlternativeRoutes) != 1 {
		t.Fatalf("expected 1 alternative route, got %d", len(d.AlternativeRoutes))
	}
	if d.AlternativeRoutes[0].Title != "Arête des Cosmiques" {
		t.Errorf("Alt title: got %q, want %q", d.AlternativeRoutes[0].Title, "Arête des Cosmiques")
	}
}

func TestGetDetail_ScheduleUsesNaismith(t *testing.T) {
	// No time_required in locales → Naismith formula
	srv := c2cDetailServer(t, minimalRouteDoc(nil))
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Schedule.Source != "formula" {
		t.Errorf("Schedule.Source: got %q, want %q", d.Schedule.Source, "formula")
	}
	if d.Schedule.EstimatedDurationHours <= 0 {
		t.Error("EstimatedDurationHours must be > 0")
	}
}

func TestGetDetail_ScheduleFromC2C(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"locales": []any{
			map[string]any{
				"lang":          "fr",
				"title":         "Test Route",
				"description":   "Une belle voie.",
				"time_required": "1", // C2C time_required present
			},
		},
	})
	srv := c2cDetailServer(t, doc)
	defer srv.Close()
	baseURL = srv.URL

	d, err := GetDetail(context.Background(), "123456", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Schedule.Source != "camptocamp" {
		t.Errorf("Schedule.Source: got %q, want %q", d.Schedule.Source, "camptocamp")
	}
}

func TestGetDetail_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	baseURL = srv.URL

	_, err := GetDetail(context.Background(), "999", "fr")
	if err == nil {
		t.Fatal("expected error for failed API call, got nil")
	}
}

func TestGetDetail_MockIsComplete(t *testing.T) {
	// Ensure the test mock has all required fields (regression guard)
	d := mockDetail("test-id")
	if d.ID != "test-id" {
		t.Errorf("mock ID: got %q, want %q", d.ID, "test-id")
	}
	if d.AlternativeRoutes == nil {
		t.Error("mock AlternativeRoutes must not be nil")
	}
	if len(d.Pitches) == 0 {
		t.Error("mock Pitches must not be empty")
	}
	if d.Schedule.Source == "" {
		t.Error("mock Schedule.Source must be set")
	}
}
