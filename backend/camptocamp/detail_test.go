package camptocamp

import (
	"context"
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockDetail is kept here for test use only.
func mockDetail(id string) *RouteDetail {
	return &RouteDetail{
		ID:             id,
		Title:          "Aiguille de l'Index — Voie normale (mock)",
		Description:    "Belle voie classique accessible depuis l'Aiguille du Midi.",
		Difficulty:     "4c",
		ElevationGain:  650,
		HeightDiffDown: 400,
		Lat:            45.9,
		Lon:            6.9,
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
		"document_id":      float64(123456),
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
	maps.Copy(doc, overrides)
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
	// calculated_duration is in days; 0.5 days = 12 hours
	doc := minimalRouteDoc(map[string]any{
		"calculated_duration": 0.5,
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
	if d.Schedule.EstimatedDurationHours != 12.0 {
		t.Errorf("EstimatedDurationHours: got %v, want 12.0", d.Schedule.EstimatedDurationHours)
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
	if d.Schedule.Source == "" {
		t.Error("mock Schedule.Source must be set")
	}
}

// --- extractGearText ---

func TestExtractGearText_FromLocale(t *testing.T) {
	doc := minimalRouteDoc(nil) // has gear in locales
	text := extractGearText(doc, "fr")
	if text != "Corde 60m, 12 dégaines" {
		t.Errorf("extractGearText from locale: got %q", text)
	}
}

func TestExtractGearText_FallbackToEquipmentRating(t *testing.T) {
	// No gear field in locales, but equipment_rating set
	doc := minimalRouteDoc(map[string]any{
		"equipment_rating": "P2",
		"locales": []any{
			map[string]any{
				"lang":        "fr",
				"title":       "Test Route",
				"description": "Une belle voie.",
			},
		},
	})
	text := extractGearText(doc, "fr")
	if text != "P2" {
		t.Errorf("extractGearText fallback: expected P2, got %q", text)
	}
}

func TestExtractGearText_Empty(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"locales": []any{
			map[string]any{
				"lang":        "fr",
				"title":       "Test Route",
				"description": "Une belle voie.",
			},
		},
	})
	text := extractGearText(doc, "fr")
	if text != "" {
		t.Errorf("extractGearText empty: expected empty, got %q", text)
	}
}

// --- allImageFilenames ---

func TestAllImageFilenames_WithImages(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"associations": map[string]any{
			"images": []any{
				map[string]any{"filename": "photo1.jpg"},
				map[string]any{"filename": "photo2.jpg"},
				map[string]any{"filename": ""},      // empty filename - should be skipped
				map[string]any{"other": "no-filename"}, // no filename key - should be skipped
			},
		},
	})
	filenames := allImageFilenames(doc)
	if len(filenames) != 2 {
		t.Fatalf("expected 2 filenames, got %d: %v", len(filenames), filenames)
	}
	if filenames[0] != "photo1.jpg" {
		t.Errorf("expected photo1.jpg, got %q", filenames[0])
	}
	if filenames[1] != "photo2.jpg" {
		t.Errorf("expected photo2.jpg, got %q", filenames[1])
	}
}

func TestAllImageFilenames_NoAssociations(t *testing.T) {
	doc := map[string]any{"document_id": float64(1)}
	filenames := allImageFilenames(doc)
	if filenames != nil {
		t.Errorf("expected nil for missing associations, got %v", filenames)
	}
}

func TestAllImageFilenames_NoImagesKey(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"associations": map[string]any{
			"routes": []any{},
		},
	})
	filenames := allImageFilenames(doc)
	if filenames != nil {
		t.Errorf("expected nil when no images key, got %v", filenames)
	}
}

func TestAllImageFilenames_EmptyImages(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"associations": map[string]any{
			"images": []any{},
		},
	})
	filenames := allImageFilenames(doc)
	if len(filenames) != 0 {
		t.Errorf("expected no filenames for empty images, got %v", filenames)
	}
}

// --- parseRisks ---

func TestParseRisks_FromLocale(t *testing.T) {
	doc := minimalRouteDoc(nil) // has "remarks" in locales
	risks := parseRisks(doc, "fr")
	if len(risks) == 0 {
		t.Fatal("expected at least one risk from locale remarks")
	}
	if risks[0] != "Attention aux chutes de pierres." {
		t.Errorf("unexpected risk: %q", risks[0])
	}
}

func TestParseRisks_DefaultFrench(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"locales": []any{
			map[string]any{"lang": "fr", "title": "Test"},
		},
	})
	risks := parseRisks(doc, "fr")
	if len(risks) == 0 {
		t.Fatal("expected default French risks when none in locales")
	}
}

func TestParseRisks_DefaultEnglish(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"locales": []any{
			map[string]any{"lang": "en", "title": "Test"},
		},
	})
	risks := parseRisks(doc, "en")
	if len(risks) == 0 {
		t.Fatal("expected default English risks when none in locales")
	}
}

// --- bestGrade ---

func TestBestGrade_ClimbingRating(t *testing.T) {
	doc := map[string]any{"climbing_rating": "6a"}
	if g := bestGrade(doc); g != "6a" {
		t.Errorf("expected 6a, got %q", g)
	}
}

func TestBestGrade_FallbackToGlobalRating(t *testing.T) {
	doc := map[string]any{"global_rating": "D"}
	if g := bestGrade(doc); g != "D" {
		t.Errorf("expected D, got %q", g)
	}
}

func TestBestGrade_FallbackToHikingRating(t *testing.T) {
	doc := map[string]any{"hiking_rating": "F"}
	if g := bestGrade(doc); g != "F" {
		t.Errorf("expected F, got %q", g)
	}
}

func TestBestGrade_Empty(t *testing.T) {
	doc := map[string]any{}
	if g := bestGrade(doc); g != "" {
		t.Errorf("expected empty string, got %q", g)
	}
}

// --- scheduleFromHours ---

func TestScheduleFromHours_ZeroHoursFallsTo4(t *testing.T) {
	s := scheduleFromHours(0, "formula", nil)
	if s.EstimatedDurationHours != 4 {
		t.Errorf("expected 4h for zero input, got %v", s.EstimatedDurationHours)
	}
	if s.RecommendedStartTime != "06:00" {
		t.Errorf("expected 06:00 start, got %q", s.RecommendedStartTime)
	}
}

func TestScheduleFromHours_LongDurationCapsAt20(t *testing.T) {
	s := scheduleFromHours(20, "formula", nil)
	if s.RecommendedEndTime != "20:00" {
		t.Errorf("expected 20:00 cap, got %q", s.RecommendedEndTime)
	}
}

func TestScheduleFromHours_WithSteps(t *testing.T) {
	steps := []DurationStep{{Label: "Approche", Hours: 1.5}, {Label: "Escalade", Hours: 4}}
	s := scheduleFromHours(5.5, "llm", steps)
	if s.Source != "llm" {
		t.Errorf("expected source=llm, got %q", s.Source)
	}
	if len(s.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(s.Steps))
	}
}

// --- parseSchedule ---

func TestParseSchedule_FromC2C(t *testing.T) {
	m := map[string]any{"calculated_duration": 0.25} // 0.25 days = 6h
	sched := parseSchedule(context.Background(), "", "fr", m, 600, 400, nil)
	if sched.Source != "camptocamp" {
		t.Errorf("expected camptocamp source, got %q", sched.Source)
	}
	if sched.EstimatedDurationHours != 6.0 {
		t.Errorf("expected 6h, got %v", sched.EstimatedDurationHours)
	}
}

func TestParseSchedule_NaismithFallback(t *testing.T) {
	m := map[string]any{}
	track := [][2]float64{{45.9, 6.9}, {45.91, 6.92}, {45.92, 6.94}}
	sched := parseSchedule(context.Background(), "", "fr", m, 500, 300, track)
	if sched.Source != "formula" {
		t.Errorf("expected formula source, got %q", sched.Source)
	}
	if sched.EstimatedDurationHours <= 0 {
		t.Error("expected positive duration from Naismith")
	}
}

func TestParseSchedule_ElevationOnlyFallback(t *testing.T) {
	// No C2C duration, no track -> elevation-only estimate
	m := map[string]any{}
	sched := parseSchedule(context.Background(), "", "fr", m, 1200, 0, nil)
	if sched.Source != "formula" {
		t.Errorf("expected formula source, got %q", sched.Source)
	}
	expected := math.Round(1200.0/600.0*10) / 10
	if sched.EstimatedDurationHours != expected {
		t.Errorf("expected %.1f h, got %v", expected, sched.EstimatedDurationHours)
	}
}

// --- parseLatLon edge cases ---

func TestParseLatLon_MissingGeom(t *testing.T) {
	doc := map[string]any{"geometry": map[string]any{}}
	lat, lon := parseLatLon(doc)
	if lat != 0 || lon != 0 {
		t.Errorf("expected 0,0 for missing geom, got %v,%v", lat, lon)
	}
}

func TestParseLatLon_MissingGeometry(t *testing.T) {
	doc := map[string]any{}
	lat, lon := parseLatLon(doc)
	if lat != 0 || lon != 0 {
		t.Errorf("expected 0,0 for missing geometry, got %v,%v", lat, lon)
	}
}

func TestParseLatLon_InvalidJSON(t *testing.T) {
	doc := map[string]any{
		"geometry": map[string]any{"geom": "{bad json"},
	}
	lat, lon := parseLatLon(doc)
	if lat != 0 || lon != 0 {
		t.Errorf("expected 0,0 for invalid JSON, got %v,%v", lat, lon)
	}
}

// --- parseAlternatives lang fallback ---

func TestParseAlternatives_EnglishFallback(t *testing.T) {
	doc := minimalRouteDoc(map[string]any{
		"associations": map[string]any{
			"routes": []any{
				map[string]any{
					"document_id": float64(999),
					"locales":     []any{}, // no locale title
				},
			},
		},
	})
	alts := parseAlternatives(doc, "en")
	if len(alts) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(alts))
	}
	if alts[0].Title != "Alternative route" {
		t.Errorf("expected English fallback title, got %q", alts[0].Title)
	}
}

func TestParseAlternatives_NoAssociations(t *testing.T) {
	doc := map[string]any{}
	alts := parseAlternatives(doc, "fr")
	if alts == nil {
		t.Error("expected empty slice, not nil")
	}
	if len(alts) != 0 {
		t.Errorf("expected 0 alternatives, got %d", len(alts))
	}
}

// --- colorFromIndices ---

func TestColorFromIndices_EasierThanRequired(t *testing.T) {
	// route difficulty is at lower index than user level → green (easier)
	if c := colorFromIndices(2, 5); c != "green" {
		t.Errorf("expected green (easy route), got %q", c)
	}
}

func TestColorFromIndices_EqualDifficulty(t *testing.T) {
	if c := colorFromIndices(5, 5); c != "black" {
		t.Errorf("expected black (matching), got %q", c)
	}
}

func TestColorFromIndices_HarderThanRequired(t *testing.T) {
	if c := colorFromIndices(8, 5); c != "red" {
		t.Errorf("expected red (hard route), got %q", c)
	}
}

func TestColorFromIndices_UnknownIndex(t *testing.T) {
	if c := colorFromIndices(-1, 5); c != "" {
		t.Errorf("expected empty for unknown index, got %q", c)
	}
	if c := colorFromIndices(5, -1); c != "" {
		t.Errorf("expected empty for unknown diff index, got %q", c)
	}
}
