package camptocamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- sampleTrack ---

func TestSampleTrack_FewerThanN(t *testing.T) {
	track := [][2]float64{{1, 2}, {3, 4}, {5, 6}}
	got := sampleTrack(track, 10)
	if len(got) != len(track) {
		t.Errorf("expected %d points, got %d", len(track), len(got))
	}
}

func TestSampleTrack_ExactlyN(t *testing.T) {
	track := make([][2]float64, 5)
	for i := range track {
		track[i] = [2]float64{float64(i), 0}
	}
	got := sampleTrack(track, 5)
	if len(got) != 5 {
		t.Errorf("expected 5, got %d", len(got))
	}
}

func TestSampleTrack_MoreThanN(t *testing.T) {
	track := make([][2]float64, 200)
	for i := range track {
		track[i] = [2]float64{float64(i) * 0.01, 0}
	}
	got := sampleTrack(track, 100)
	if len(got) != 100 {
		t.Errorf("expected 100, got %d", len(got))
	}
	// First and last must be preserved
	if got[0] != track[0] {
		t.Errorf("first point changed: got %v, want %v", got[0], track[0])
	}
	if got[99] != track[199] {
		t.Errorf("last point changed: got %v, want %v", got[99], track[199])
	}
}

func TestSampleTrack_SinglePoint(t *testing.T) {
	track := [][2]float64{{45.0, 6.0}}
	got := sampleTrack(track, 100)
	if len(got) != 1 {
		t.Errorf("expected 1 point, got %d", len(got))
	}
}

// --- parseTrack ---

func TestParseTrack_NoGeometry(t *testing.T) {
	m := map[string]any{}
	got := parseTrack(m)
	if got != nil {
		t.Errorf("expected nil for missing geometry, got %v", got)
	}
}

func TestParseTrack_EmptyGeomDetail(t *testing.T) {
	m := map[string]any{
		"geometry": map[string]any{
			"geom_detail": "",
		},
	}
	got := parseTrack(m)
	if got != nil {
		t.Errorf("expected nil for empty geom_detail, got %v", got)
	}
}

func TestParseTrack_LineString(t *testing.T) {
	// Two Mercator points around Chamonix
	coords := [][2]float64{{765071, 5768286}, {766000, 5769000}}
	raw, _ := json.Marshal(map[string]any{
		"type":        "LineString",
		"coordinates": coords,
	})
	m := map[string]any{
		"geometry": map[string]any{
			"geom_detail": string(raw),
		},
	}
	got := parseTrack(m)
	if len(got) != 2 {
		t.Fatalf("expected 2 points, got %d", len(got))
	}
	// Results should be valid WGS84 (lat ≈ 45.9, lon ≈ 6.87)
	if got[0][0] < 40 || got[0][0] > 50 {
		t.Errorf("lat out of range: %f", got[0][0])
	}
}

func TestParseTrack_MultiLineString(t *testing.T) {
	line1 := [][2]float64{{765071, 5768286}, {766000, 5769000}}
	line2 := [][2]float64{{767000, 5770000}, {768000, 5771000}}
	raw, _ := json.Marshal(map[string]any{
		"type":        "MultiLineString",
		"coordinates": [][][2]float64{line1, line2},
	})
	m := map[string]any{
		"geometry": map[string]any{
			"geom_detail": string(raw),
		},
	}
	got := parseTrack(m)
	if len(got) != 4 {
		t.Fatalf("expected 4 points (2+2), got %d", len(got))
	}
}

func TestParseTrack_UnknownType(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"type":        "Point",
		"coordinates": []float64{765071, 5768286},
	})
	m := map[string]any{
		"geometry": map[string]any{
			"geom_detail": string(raw),
		},
	}
	got := parseTrack(m)
	if got != nil {
		t.Errorf("expected nil for unknown GeoJSON type, got %v", got)
	}
}

func TestParseTrack_InvalidJSON(t *testing.T) {
	m := map[string]any{
		"geometry": map[string]any{
			"geom_detail": "not-json",
		},
	}
	got := parseTrack(m)
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}

// --- fetchElevationProfile ---

func TestFetchElevationProfile_EmptyTrack(t *testing.T) {
	got := fetchElevationProfile(context.Background(), nil)
	if got != nil {
		t.Errorf("expected nil for empty track, got %v", got)
	}
}

func TestFetchElevationProfile_SinglePoint(t *testing.T) {
	track := [][2]float64{{45.9, 6.9}}
	got := fetchElevationProfile(context.Background(), track)
	if got != nil {
		t.Errorf("expected nil for single-point track (needs ≥2 points), got %v", got)
	}
}

func TestFetchElevationProfile_MockOpenTopoData(t *testing.T) {
	// Build a 3-point track
	track := [][2]float64{
		{45.90, 6.90},
		{45.91, 6.91},
		{45.92, 6.92},
	}

	// Mock OpenTopoData server
	elev1, elev2, elev3 := 1000.0, 1200.0, 1100.0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{"elevation": elev1},
				map[string]any{"elevation": elev2},
				map[string]any{"elevation": elev3},
			},
		})
	}))
	defer srv.Close()

	// Patch the URL used by fetchElevationProfile via the opentopodata URL
	// Since the URL is hardcoded, we swap the http.DefaultClient transport
	origTransport := http.DefaultTransport
	http.DefaultTransport = &prefixRewriter{target: srv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	got := fetchElevationProfile(context.Background(), track)
	if len(got) != 3 {
		t.Fatalf("expected 3 profile points, got %d", len(got))
	}
	if got[0][0] != 0 {
		t.Errorf("first distance should be 0, got %f", got[0][0])
	}
	if got[0][1] != elev1 {
		t.Errorf("first elevation: got %f, want %f", got[0][1], elev1)
	}
	if got[1][1] != elev2 {
		t.Errorf("second elevation: got %f, want %f", got[1][1], elev2)
	}
}

func TestFetchElevationProfile_ServerError(t *testing.T) {
	track := [][2]float64{{45.9, 6.9}, {45.91, 6.91}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &prefixRewriter{target: srv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	got := fetchElevationProfile(context.Background(), track)
	if got != nil {
		t.Errorf("expected nil on server error, got %v", got)
	}
}

func TestFetchElevationProfile_NilElevation(t *testing.T) {
	track := [][2]float64{{45.9, 6.9}, {45.91, 6.91}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Elevation is null
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []any{
				map[string]any{"elevation": nil},
				map[string]any{"elevation": nil},
			},
		})
	}))
	defer srv.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &prefixRewriter{target: srv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	got := fetchElevationProfile(context.Background(), track)
	if got != nil {
		t.Errorf("expected nil when elevation is null, got %v", got)
	}
}

// prefixRewriter redirects all requests to the given HTTP test server.
type prefixRewriter struct {
	target string
	base   http.RoundTripper
}

func (p *prefixRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	// Extract host:port from target "http://127.0.0.1:PORT"
	host := p.target
	if len(host) > 7 {
		host = host[7:] // strip "http://"
	}
	cloned.URL.Host = host
	return p.base.RoundTrip(cloned)
}

// --- haversineKm ---

func TestHaversineKm(t *testing.T) {
	// Paris to Bordeaux is roughly 500 km
	paris := [2]float64{48.8566, 2.3522}
	bordeaux := [2]float64{44.8378, -0.5792}
	d := haversineKm(paris, bordeaux)
	if d < 480 || d > 520 {
		t.Errorf("haversineKm Paris-Bordeaux: got %.1f km, expected ~500 km", d)
	}
}

func TestHaversineKm_SamePoint(t *testing.T) {
	p := [2]float64{45.9, 6.9}
	d := haversineKm(p, p)
	if d != 0 {
		t.Errorf("expected 0 for same point, got %f", d)
	}
}
