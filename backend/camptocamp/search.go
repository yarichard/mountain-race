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
	"multipitch": "rock_climbing",
	"ridge_hike": "mountain_climbing",
	"hike":       "hiking",
}

// climbingGradeOrder is the ordered French sport climbing scale used for comparisons.
var climbingGradeOrder = []string{
	"3a", "3b", "3c",
	"4a", "4b", "4c",
	"5a", "5b", "5c",
	"6a", "6a+", "6b", "6b+", "6c", "6c+",
	"7a", "7a+", "7b", "7b+", "7c", "7c+",
	"8a", "8a+", "8b", "8b+", "8c", "8c+",
	"9a", "9b", "9c",
}

// alpineToClimbing maps alpine cotation to its French climbing grade equivalent.
var alpineToClimbing = map[string]string{
	"F":  "3c",
	"PD": "4c",
	"AD": "5c",
	"D":  "6b+",
	"TD": "7b",
	"ED": "8a+",
}

// normalizeGrade converts C2C's internal suffix (_sup) to the standard display format (+).
func normalizeGrade(g string) string {
	return strings.ReplaceAll(g, "_sup", "+")
}

func gradeIndex(g string) int {
	g = normalizeGrade(g)
	for i, v := range climbingGradeOrder {
		if v == g {
			return i
		}
	}
	return -1
}

var alpineGradeOrder = []string{"F", "PD", "AD", "D", "TD", "ED"}

func alpineGradeIndex(g string) int {
	for i, v := range alpineGradeOrder {
		if v == g {
			return i
		}
	}
	return -1
}

// colorFromIndices returns "green", "black", or "red" given pre-computed ordinal indices.
// Returns "" when either index is unknown (-1).
func colorFromIndices(routeIdx, diffIdx int) string {
	if routeIdx < 0 || diffIdx < 0 {
		return ""
	}
	if routeIdx < diffIdx {
		return "green"
	}
	if routeIdx == diffIdx {
		return "black"
	}
	return "red"
}

// SearchResult is a lightweight route summary.
type SearchResult struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	Difficulty      string `json:"difficulty"`
	DifficultyColor string `json:"difficulty_color"`
	HeightDiffUp    int    `json:"height_diff_up"`
	HeightDiffDown  int    `json:"height_diff_down"`
	SourceURL       string `json:"source_url"`
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
	AllowAbove   bool          `json:"allow_above"`
	Date         string        `json:"date"`
	Lang         string        `json:"lang"` // preferred display language, e.g. "fr" or "en"
	Participants []Participant `json:"participants"`
	RadiusKm     int           `json:"radius_km"` // search radius in km for location-based search (default 20)
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
			radiusKm := req.RadiusKm
			if radiusKm <= 0 {
				radiusKm = 20
			}
			radiusM := float64(radiusKm) * 1000.0
			params.Set("bbox", fmt.Sprintf("%.0f,%.0f,%.0f,%.0f", x-radiusM, y-radiusM, x+radiusM, y+radiusM))
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
		locs := localesField(m)
		title := pickLocale(locs, req.Lang, "title")
		title += "("+ pickLocale(locs, req.Lang, "title_prefix") +")"
		difficulty, color := gradeFromDoc(m, req.RaceType, req.Difficulty)
		if color == "red" && !req.AllowAbove {
			continue // route is harder than the selected difficulty
		}
		results = append(results, SearchResult{
			ID:              id,
			Title:           title,
			Summary:         pickLocale(locs, req.Lang, "summary"),
			Difficulty:      difficulty,
			DifficultyColor: color,
			HeightDiffUp:    intField(m, "height_diff_up"),
			HeightDiffDown:  intField(m, "height_diff_down"),
			SourceURL:       "https://www.camptocamp.org/routes/" + id,
		})
	}

	return results, nil
}


// gradeFromDoc builds the display grade string for a route and computes a color by comparing
// the route's key rating against selectedDifficulty (the value chosen in the search form).
// For multipitch: compares French sport grades. For hike/ridge_hike: compares alpine cotation directly.
func gradeFromDoc(m map[string]any, raceType, selectedDifficulty string) (grade, color string) {
	var parts []string
	addPart := func(key string) {
		if v := normalizeGrade(stringField(m, key)); v != "" {
			parts = append(parts, v)
		}
	}
	addPart("global_rating")
	free := normalizeGrade(stringField(m, "rock_free_rating"))
	req := normalizeGrade(stringField(m, "rock_required_rating"))
	switch {
	case free != "" && req != "":
		parts = append(parts, free+" > "+req)
	case free != "":
		parts = append(parts, free)
	case req != "":
		parts = append(parts, req)
	}
	addPart("engagement_rating")
	addPart("equipment_rating")
	addPart("exposition_rock_rating")
	grade = strings.Join(parts, " ")

	if raceType == "multipitch" {
		routeGrade := normalizeGrade(stringField(m, "rock_required_rating"))
		if routeGrade == "" {
			routeGrade = normalizeGrade(stringField(m, "rock_free_rating"))
		}
		color = colorFromIndices(gradeIndex(routeGrade), gradeIndex(selectedDifficulty))
	} else {
		routeGrade := stringField(m, "global_rating")
		color = colorFromIndices(alpineGradeIndex(routeGrade), alpineGradeIndex(selectedDifficulty))
	}
	return grade, color
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
