package pdf

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
	}{
		{
			name:     "bold text",
			input:    "**L1** Première longueur",
			contains: []string{"<strong>L1</strong>"},
		},
		{
			name:     "h2 heading",
			input:    "## Section",
			contains: []string{"<h3>Section</h3>"},
		},
		{
			name:     "h3 heading",
			input:    "### Sous-section",
			contains: []string{"<h4>Sous-section</h4>"},
		},
		{
			name:     "double newline becomes paragraph break",
			input:    "Premier\n\nSecond",
			contains: []string{"</p><p>"},
		},
		{
			name:     "single newline becomes br",
			input:    "Ligne 1\nLigne 2",
			contains: []string{"<br>"},
		},
		{
			name:     "HTML in input is escaped",
			input:    "<script>alert(1)</script>",
			absent:   []string{"<script>"},
			contains: []string{"&lt;script&gt;"},
		},
		{
			name:     "ampersand escaped",
			input:    "Snow & Ice",
			contains: []string{"Snow &amp; Ice"},
		},
		{
			name:     "wrapped in paragraph tags",
			input:    "text",
			contains: []string{"<p>", "</p>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(markdownToHTML(tt.input))
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got: %s", want, result)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(result, absent) {
					t.Errorf("expected output NOT to contain %q, got: %s", absent, result)
				}
			}
		})
	}
}

func TestBuildElevationSVG(t *testing.T) {
	t.Run("empty profile returns empty string", func(t *testing.T) {
		result := buildElevationSVG(nil)
		if result != "" {
			t.Errorf("expected empty, got: %s", result)
		}
		result = buildElevationSVG([][2]float64{{0, 1000}})
		if result != "" {
			t.Errorf("expected empty for single point, got: %s", result)
		}
	})

	t.Run("valid profile produces SVG", func(t *testing.T) {
		profile := [][2]float64{
			{0, 1000},
			{1, 1200},
			{2, 1400},
			{3, 1350},
			{4, 1100},
		}
		result := string(buildElevationSVG(profile))
		if !strings.Contains(result, "<svg") {
			t.Error("expected SVG element")
		}
		if !strings.Contains(result, "<polyline") {
			t.Error("expected polyline element")
		}
		if !strings.Contains(result, "<polygon") {
			t.Error("expected polygon fill element")
		}
		if !strings.Contains(result, "1400m") {
			t.Error("expected max elevation label")
		}
		if !strings.Contains(result, "1000m") {
			t.Error("expected min elevation label")
		}
		if !strings.Contains(result, "4.0km") {
			t.Error("expected max distance label")
		}
	})

	t.Run("flat profile does not panic", func(t *testing.T) {
		profile := [][2]float64{{0, 2000}, {5, 2000}}
		result := buildElevationSVG(profile)
		if result == "" {
			t.Error("expected non-empty SVG for flat profile")
		}
	})
}

func TestBuildMapSVG(t *testing.T) {
	// buildMapSVG tries OSM tiles first; in these tests we point osmTileBase
	// at a server that returns 404 so the fallback SVG path is exercised.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv404.Close()

	original := osmTileBase
	osmTileBase = srv404.URL
	defer func() { osmTileBase = original }()

	t.Run("no track and no lat/lon returns empty", func(t *testing.T) {
		result := buildMapSVG(nil, 0, 0)
		if result != "" {
			t.Errorf("expected empty, got: %s", result)
		}
	})

	t.Run("single lat/lon falls back to plain SVG with pin", func(t *testing.T) {
		result := string(buildMapSVG(nil, 45.9, 6.9))
		if !strings.Contains(result, "<svg") {
			t.Error("expected SVG element")
		}
		if !strings.Contains(result, "<circle") {
			t.Error("expected circle pin marker")
		}
	})

	t.Run("full track falls back to plain SVG with polyline and markers", func(t *testing.T) {
		track := [][2]float64{
			{45.8, 6.8},
			{45.85, 6.85},
			{45.9, 6.9},
		}
		result := string(buildMapSVG(track, 45.85, 6.85))
		if !strings.Contains(result, "<polyline") {
			t.Error("expected polyline for track")
		}
		if strings.Count(result, "<circle") < 4 {
			t.Error("expected at least 4 circle elements (2 markers × 2 circles)")
		}
	})
}

func TestBuildMapSVGWithTiles(t *testing.T) {
	// Serve a minimal 1×1 white PNG (smallest valid PNG).
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk length + type
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1×1 pixels
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB, CRC
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND chunk
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	tileSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(minimalPNG) //nolint:errcheck
	}))
	defer tileSrv.Close()

	original := osmTileBase
	osmTileBase = tileSrv.URL
	defer func() { osmTileBase = original }()

	t.Run("tile fetch succeeds: SVG contains <image> elements", func(t *testing.T) {
		track := [][2]float64{
			{45.8, 6.8},
			{45.85, 6.85},
			{45.9, 6.9},
		}
		result := string(buildMapSVG(track, 45.85, 6.85))
		if !strings.Contains(result, "<image ") {
			t.Errorf("expected <image> elements from tile fetch, got: %.200s", result)
		}
		if !strings.Contains(result, "data:image/png;base64,") {
			t.Error("expected base64-encoded tile PNG")
		}
		if !strings.Contains(result, "<polyline") {
			t.Error("expected route polyline overlay")
		}
	})

	t.Run("zoom and tile math: selectZoom returns sane value", func(t *testing.T) {
		z := selectZoom(45.8, 45.9, 6.8, 6.9, 3, 2)
		if z < 8 || z > 15 {
			t.Errorf("unexpected zoom %d", z)
		}
	})

	t.Run("latLonToTileXY: known Chamonix coordinates", func(t *testing.T) {
		// Chamonix at zoom 13: x≈4253, y≈2940 (approximate)
		tx, ty := latLonToTileXY(45.924, 6.869, 13)
		if tx < 4200 || tx > 4300 {
			t.Errorf("unexpected tile x=%d for Chamonix", tx)
		}
		if ty < 2900 || ty > 3000 {
			t.Errorf("unexpected tile y=%d for Chamonix", ty)
		}
	})
}

func TestBuildImagesHTML(t *testing.T) {
	t.Run("empty list returns empty", func(t *testing.T) {
		result := buildImagesHTML(nil)
		if result != "" {
			t.Errorf("expected empty, got: %s", result)
		}
	})

	t.Run("fetches and embeds image as base64 data URI", func(t *testing.T) {
		// Minimal JPEG magic bytes.
		fakeJPEG := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write(fakeJPEG) //nolint:errcheck
		}))
		defer srv.Close()

		original := c2cImageBase
		c2cImageBase = srv.URL + "/"
		defer func() { c2cImageBase = original }()

		result := string(buildImagesHTML([]string{"test.jpg"}))
		if !strings.Contains(result, "data:image/jpeg;base64,") {
			t.Errorf("expected base64 data URI, got: %s", result)
		}
		if !strings.Contains(result, "<img ") {
			t.Errorf("expected img tag, got: %s", result)
		}
	})

	t.Run("caps at maxImages", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8}) //nolint:errcheck
		}))
		defer srv.Close()

		original := c2cImageBase
		c2cImageBase = srv.URL + "/"
		defer func() { c2cImageBase = original }()

		// Supply more than maxImages filenames.
		names := make([]string, maxImages+3)
		for i := range names {
			names[i] = "img.jpg"
		}
		result := string(buildImagesHTML(names))
		count := strings.Count(result, "<img ")
		if count > maxImages {
			t.Errorf("expected at most %d images, got %d", maxImages, count)
		}
	})

	t.Run("failed fetch is silently skipped", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		original := c2cImageBase
		c2cImageBase = srv.URL + "/"
		defer func() { c2cImageBase = original }()

		result := buildImagesHTML([]string{"missing.jpg"})
		if result != "" {
			t.Errorf("expected empty result for failed fetch, got: %s", result)
		}
	})
}

func TestExportTemplateRender(t *testing.T) {
	plan := PlanData{
		Title:          "Test Route",
		Difficulty:     "5c",
		ElevationGain:  500,
		HeightDiffDown: 500,
		DistanceKm:     5.5,
		Description:    "**L1** Première longueur\n**L2** Deuxième longueur",
		Participants: []ParticipantData{
			{Name: "Alice", ClimbingLevel: "6a"},
			{Name: "Bob", ClimbingLevel: "5c"},
		},
		Objectives: []string{"challenge", "fun"},
		Notes:      "Notes de test",
		Equipment: []EquipmentData{
			{Item: "Corde", Quantity: 1, Notes: "obligatoire"},
		},
		Risks: []string{"Risque de chute de pierres"},
		AlternativeRoutes: []AlternativeRoute{
			{ID: "123", Title: "Voie alternative", Reason: "Repli facile"},
		},
		Schedule: ScheduleData{
			EstimatedDurationHours: 4,
			RecommendedStartTime:   "06:00",
			RecommendedEndTime:     "14:00",
			Source:                 "formula",
		},
		Weather: WeatherData{
			Forecast: ForecastData{
				TemperatureMin: 5,
				TemperatureMax: 18,
				Precipitation:  0,
				WindSpeedKmh:   20,
			},
			Avalanche: AvalancheData{
				RiskLevel: 2,
				RiskLabel: "Limité",
			},
		},
		ElevationProfile: [][2]float64{
			{0, 1000}, {1, 1200}, {2, 1400}, {3, 1200},
		},
		Track: [][2]float64{
			{45.8, 6.8}, {45.85, 6.85}, {45.9, 6.9},
		},
		Lat:         45.85,
		Lon:         6.85,
		GeneratedAt: "27/05/2026 10:00",
	}
	plan.DescriptionHTML = markdownToHTML(plan.Description)
	plan.ElevationSVG = buildElevationSVG(plan.ElevationProfile)
	plan.MapSVG = buildMapSVG(plan.Track, 45.85, 6.85)

	tmpl, err := template.New("test").Parse(exportTemplate)
	if err != nil {
		t.Fatalf("template parse error: %v", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, plan); err != nil {
		t.Fatalf("template execute error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Test Route",
		"5c",
		"500",
		"Alice",
		"Bob",
		"Corde",
		"Risque de chute de pierres",
		"Voie alternative",
		"Profil altimétrique",
		"Carte",
		"challenge",
		"fun",
		"Notes de test",
		"Naismith",
		"GPS",
		"45.85000",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected HTML to contain %q", want)
		}
	}
	if !strings.Contains(out, "<strong>L1</strong>") {
		t.Error("expected Markdown bold to be converted to <strong>")
	}
}
