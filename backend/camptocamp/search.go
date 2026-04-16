package camptocamp

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RouteType maps our internal race type to C2C activity codes.
var activityMap = map[string]string{
	"multipitch":  "rock_climbing",
	"ridge_hike":  "mountain_climbing",
	"hike":        "hiking",
}

// SearchResult is a lightweight route summary.
type SearchResult struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	Difficulty   string  `json:"difficulty"`
	ElevationGain int    `json:"elevation_gain"`
	DistanceKm   float64 `json:"distance_km"`
	SourceURL    string  `json:"source_url"`
}

// Participant holds a single person's details.
type Participant struct {
	Name          string `json:"name"`
	ClimbingLevel string `json:"climbing_level"`
}

// SearchRequest holds the user search criteria.
type SearchRequest struct {
	Location     string        `json:"location"`
	LocationType string        `json:"location_type"` // "name" | "location"
	RaceType     string        `json:"race_type"`
	Difficulty   string        `json:"difficulty"`
	Date         string        `json:"date"`
	Participants []Participant `json:"participants"`
}

// Search queries CampToCamp for routes matching the criteria.
func Search(req SearchRequest) ([]SearchResult, error) {
	act, ok := activityMap[req.RaceType]
	if !ok {
		act = "rock_climbing"
	}

	params := url.Values{}
	params.Set("act", act)
	params.Set("limit", "20")
	params.Set("offset", "0")

	if req.LocationType == "location" && req.Location != "" {
		lat, lon, err := geocodeLocation(req.Location)
		if err == nil {
			x, y := wgs84ToMercator(lat, lon)
			const radius = 20000.0 // 20 km
			params.Set("bbox", fmt.Sprintf("%.0f,%.0f,%.0f,%.0f", x-radius, y-radius, x+radius, y+radius))
		}
	} else if req.Location != "" {
		params.Set("q", req.Location)
	}

	data, err := get("/routes?" + params.Encode())
	if err != nil {
		return nil, err
	}

	docs, _ := data["documents"].([]any)
	results := []SearchResult{}
	for _, doc := range docs {
		m, ok := doc.(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprintf("%.0f", floatField(m, "document_id"))
		title := localizedString(m, "locales")
		if title == "" {
			title = firstLocaleTitle(m)
		}
		difficulty := gradeFromDoc(m, req.RaceType)
		results = append(results, SearchResult{
			ID:            id,
			Title:         title,
			Summary:       summaryFromDoc(m),
			Difficulty:    difficulty,
			ElevationGain: intField(m, "elevation_gain_up"),
			DistanceKm:    floatField(m, "route_length") / 1000,
			SourceURL:     "https://www.camptocamp.org/routes/" + id,
		})
	}

	return results, nil
}

func firstLocaleTitle(m map[string]any) string {
	locales, ok := m["locales"].([]any)
	if !ok {
		return ""
	}
	for _, l := range locales {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := lm["title"].(string); ok && t != "" {
			return t
		}
	}
	return ""
}

func summaryFromDoc(m map[string]any) string {
	locales, ok := m["locales"].([]any)
	if !ok {
		return ""
	}
	for _, l := range locales {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := lm["summary"].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func gradeFromDoc(m map[string]any, raceType string) string {
	if raceType == "multipitch" {
		return stringField(m, "climbing_rating")
	}
	return stringField(m, "global_rating")
}

var nominatimClient = &http.Client{Timeout: 10 * time.Second}
var nominatimBaseURL = "https://nominatim.openstreetmap.org"

// geocodeLocation converts a place name or "lat,lon" string to WGS84 coordinates.
func geocodeLocation(location string) (lat, lon float64, err error) {
	// Try parsing as "lat,lon" first
	parts := strings.SplitN(location, ",", 2)
	if len(parts) == 2 {
		la, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lo, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if e1 == nil && e2 == nil {
			return la, lo, nil
		}
	}

	// Geocode with Nominatim (OpenStreetMap)
	u := nominatimBaseURL + "/search?format=json&limit=1&q=" + url.QueryEscape(location)
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "mountain-race-app/1.0")
	resp, err := nominatimClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}
	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.Unmarshal(body, &results); err != nil || len(results) == 0 {
		return 0, 0, fmt.Errorf("no results for %q", location)
	}
	lat, _ = strconv.ParseFloat(results[0].Lat, 64)
	lon, _ = strconv.ParseFloat(results[0].Lon, 64)
	return lat, lon, nil
}

// wgs84ToMercator converts WGS84 lat/lon to EPSG:3857 (Web Mercator) x/y.
func wgs84ToMercator(lat, lon float64) (x, y float64) {
	const R = 6378137.0
	x = lon * math.Pi / 180.0 * R
	y = math.Log(math.Tan(math.Pi/4.0+lat*math.Pi/180.0/2.0)) * R
	return x, y
}
