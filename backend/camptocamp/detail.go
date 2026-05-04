package camptocamp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"mountain-race/llm"
)

// equipExtract can be replaced in tests to avoid a real Ollama call.
var equipExtract = func(ctx context.Context, gearText, lang string) ([]llm.EquipmentItem, error) {
	return llm.ExtractEquipment(ctx, gearText, lang)
}

// Pitch represents a single pitch on a multipitch route.
type Pitch struct {
	Number      int    `json:"number"`
	Grade       string `json:"grade"`
	Description string `json:"description"`
}

// Equipment item.
type Equipment struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}

// AlternativeRoute is a related route.
type AlternativeRoute struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// Schedule holds timing information.
type Schedule struct {
	EstimatedDurationHours float64 `json:"estimated_duration_hours"`
	RecommendedStartTime   string  `json:"recommended_start_time"`
	RecommendedEndTime     string  `json:"recommended_end_time"`
	Source                 string  `json:"source"` // "camptocamp" | "formula"
}

// RouteDetail is the full route information.
type RouteDetail struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	Difficulty        string             `json:"difficulty"`
	ElevationGain     int                `json:"elevation_gain"`
	HeightDiffDown    int                `json:"height_diff_down"`
	Lat               float64            `json:"lat"`
	Lon               float64            `json:"lon"`
	Track             [][2]float64       `json:"track,omitempty"`             // WGS84 [lat, lon] pairs
	ElevationProfile  [][2]float64       `json:"elevation_profile,omitempty"` // [distance_km, elevation_m] pairs
	Pitches           []Pitch            `json:"pitches,omitempty"`
	Images            []string           `json:"images,omitempty"`
	GpxURL            string             `json:"gpx_url"`
	GearText          string             `json:"gear_text"`
	Equipment         []Equipment        `json:"equipment"`
	Risks             []string           `json:"risks"`
	AlternativeRoutes []AlternativeRoute `json:"alternative_routes"`
	Schedule          Schedule           `json:"schedule"`
	SourceURL         string             `json:"source_url"`
}


// GetDetail fetches full route detail from CampToCamp.
func GetDetail(ctx context.Context, id, lang string) (*RouteDetail, error) {
	data, err := get("/routes/" + id)
	if err != nil {
		return nil, err
	}

	locs := localesField(data)
	title := pickLocale(locs, lang, "title_prefix") + " / " + pickLocale(locs, lang, "title")
	description := pickLocale(locs, lang, "route_history") + "\n"
	description += pickLocale(locs, lang, "summary") + "\n"
	description += pickLocale(locs, lang, "description") + "\n"
	if description == "" {
		description = pickLocale(locs, lang, "route_history") + "\n"
	}
	description += pickLocale(locs, lang, "external_resources") + "\n"

	// CamptoCamp specific: some descriptions have "##" without space
	description = strings.Replace(description, "##", "## ", -1)
	// replace all "L#" occurences and replace # with L and an index beginning at 1 and increasing for each occurence.
	pitchIndex := 1
	description = regexp.MustCompile(`L#`).ReplaceAllStringFunc(description, func(s string) string {
		result := fmt.Sprintf("\n**L%d** ", pitchIndex)
		pitchIndex++
		return result
	})
	// replace all "L#" occurences and replace # with L and an index beginning at 1 and increasing for each occurence.
	relayIndex := 1
	description = regexp.MustCompile(`R#`).ReplaceAllStringFunc(description, func(s string) string {
		result := fmt.Sprintf("R%d", relayIndex)
		relayIndex++
		return result
	})

	difficulty := bestGrade(data)
	elevGain := intField(data, "height_diff_up")
	elevDown := intField(data, "height_diff_down")

	pitches := parsePitches(data, lang)

	gearText := extractGearText(data, lang)

	risks := parseRisks(data, lang)
	alts := parseAlternatives(data, lang)
	lat, lon := parseLatLon(data)
	track := parseTrack(data)
	elevProfile := fetchElevationProfile(ctx, track)

	// Schedule: check if C2C has duration data in comments/description
	sched := parseSchedule(data, float64(elevGain))

	// Images and GPX
	images := allImageFilenames(data)
	gpxURL := ""
	if as, ok := data["associations"].(map[string]any); ok {
		if docs, ok := as["waypoints"].([]any); ok && len(docs) > 0 {
			_ = docs // GPX not directly exposed via API
		}
	}

	return &RouteDetail{
		ID:                id,
		Title:             title,
		Description:       description,
		Difficulty:        difficulty,
		ElevationGain:     elevGain,
		HeightDiffDown:    elevDown,
		Lat:               lat,
		Lon:               lon,
		Track:             track,
		ElevationProfile:  elevProfile,
		Pitches:           pitches,
		Images:            images,
		GpxURL:            gpxURL,
		GearText:          gearText,
		Equipment:         []Equipment{},
		Risks:             risks,
		AlternativeRoutes: alts,
		Schedule:          sched,
		SourceURL:         "https://www.camptocamp.org/routes/" + id,
	}, nil
}

func bestGrade(m map[string]any) string {
	for _, key := range []string{"climbing_rating", "global_rating", "hiking_rating"} {
		if g := stringField(m, key); g != "" {
			return g
		}
	}
	return ""
}

func parsePitches(m map[string]any, lang string) []Pitch {
	locs := localesField(m)
	pitchText := pickLocale(locs, lang, "pitch")
	if pitchText == "" {
		return nil
	}
	return []Pitch{{Number: 1, Grade: bestGrade(m), Description: pitchText}}
}

// extractGearText returns the raw gear description text from the C2C document.
func extractGearText(m map[string]any, lang string) string {
	if text := pickLocale(localesField(m), lang, "gear"); text != "" {
		return text
	}
	if er := stringField(m, "equipment_rating"); er != "" {
		return er
	}
	return ""
}

func parseRisks(m map[string]any, lang string) []string {
	locs := localesField(m)
	var risks []string
	for _, field := range []string{"remarks", "risk"} {
		if v := pickLocale(locs, lang, field); v != "" {
			risks = append(risks, v)
		}
	}
	if len(risks) == 0 {
		if lang == "en" {
			risks = []string{"Check weather conditions before departure", "Bring enough water"}
		} else {
			risks = []string{"Vérifier les conditions météo avant de partir", "Emporter de quoi s'hydrater"}
		}
	}
	return risks
}

func parseAlternatives(m map[string]any, lang string) []AlternativeRoute {
	alts := []AlternativeRoute{}
	assoc, ok := m["associations"].(map[string]any)
	if !ok {
		return alts
	}
	routes, ok := assoc["routes"].([]any)
	if !ok {
		return alts
	}
	fallbackTitle := "Itinéraire alternatif"
	fallbackReason := "Itinéraire alternatif"
	if lang == "en" {
		fallbackTitle = "Alternative route"
		fallbackReason = "Alternative route"
	}
	for _, r := range routes {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprintf("%.0f", floatField(rm, "document_id"))
		title := pickLocale(localesField(rm), lang, "title")
		if title == "" {
			title = fallbackTitle
		}
		alts = append(alts, AlternativeRoute{
			ID:     id,
			Title:  title,
			Reason: fallbackReason,
		})
	}
	return alts
}

func parseSchedule(m map[string]any, elevGainM float64) Schedule {
	// Look for duration in locales
	locales, _ := m["locales"].([]any)
	for _, l := range locales {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if d, ok := lm["time_required"].(string); ok && d != "" {
			return Schedule{
				EstimatedDurationHours: 6,
				RecommendedStartTime:   "06:00",
				RecommendedEndTime:     "16:00",
				Source:                 "camptocamp",
			}
		}
	}

	// Naismith fallback: elevation only
	duration := elevGainM / 600.0
	if duration < 1 {
		duration = 4
	}
	endHour := min(6+int(duration), 20)

	return Schedule{
		EstimatedDurationHours: duration,
		RecommendedStartTime:   "06:00",
		RecommendedEndTime:     fmt.Sprintf("%02d:00", endHour),
		Source:                 "formula",
	}
}

func allImageFilenames(m map[string]any) []string {
	assoc, ok := m["associations"].(map[string]any)
	if !ok {
		return nil
	}
	imgs, ok := assoc["images"].([]any)
	if !ok {
		return nil
	}
	var filenames []string
	for _, item := range imgs {
		im, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if fn, ok := im["filename"].(string); ok && fn != "" {
			filenames = append(filenames, fn)
		}
	}
	return filenames
}

// webMercatorToWGS84 converts EPSG:3857 (x, y) to WGS84 (lat, lon).
func webMercatorToWGS84(x, y float64) (lat, lon float64) {
	const R = 6378137.0
	lon = x / R * (180.0 / math.Pi)
	lat = (2*math.Atan(math.Exp(y/R)) - math.Pi/2) * (180.0 / math.Pi)
	return lat, lon
}

// parseLatLon extracts WGS84 coordinates from the C2C geometry Point field.
func parseLatLon(m map[string]any) (lat, lon float64) {
	geomObj, ok := m["geometry"].(map[string]any)
	if !ok {
		return 0, 0
	}
	geomStr, ok := geomObj["geom"].(string)
	if !ok || geomStr == "" {
		return 0, 0
	}
	var geojson struct {
		Coordinates [2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geomStr), &geojson); err != nil {
		return 0, 0
	}
	return webMercatorToWGS84(geojson.Coordinates[0], geojson.Coordinates[1])
}

// fetchElevationProfile queries OpenTopoData SRTM 30m for the elevation of each
// track point and returns [distance_km, elevation_m] pairs. Returns nil on any
// failure so the caller can fall back to a synthetic profile.
func fetchElevationProfile(ctx context.Context, track [][2]float64) [][2]float64 {
	if len(track) < 2 {
		return nil
	}

	// Sample up to 100 points (API limit per request).
	sampled := sampleTrack(track, 100)

	// Build locations query string "lat,lon|lat,lon|..."
	var sb strings.Builder
	for i, p := range sampled {
		if i > 0 {
			sb.WriteByte('|')
		}
		sb.WriteString(fmt.Sprintf("%f,%f", p[0], p[1]))
	}

	url := "https://api.opentopodata.org/v1/srtm30m?locations=" + sb.String()
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	var body struct {
		Results []struct {
			Elevation *float64 `json:"elevation"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil
	}
	if len(body.Results) != len(sampled) {
		return nil
	}

	// Build profile with cumulative haversine distance.
	profile := make([][2]float64, len(sampled))
	var cumDist float64
	for i, res := range body.Results {
		if res.Elevation == nil {
			return nil
		}
		if i > 0 {
			cumDist += haversineKm(sampled[i-1], sampled[i])
		}
		profile[i] = [2]float64{cumDist, *res.Elevation}
	}
	return profile
}

// sampleTrack returns at most n evenly-spaced points from track.
func sampleTrack(track [][2]float64, n int) [][2]float64 {
	if len(track) <= n {
		return track
	}
	sampled := make([][2]float64, n)
	for i := range sampled {
		idx := int(math.Round(float64(i) * float64(len(track)-1) / float64(n-1)))
		sampled[i] = track[idx]
	}
	return sampled
}

// haversineKm returns the great-circle distance in km between two WGS84 [lat,lon] points.
func haversineKm(a, b [2]float64) float64 {
	const R = 6371.0
	lat1, lon1 := a[0]*math.Pi/180, a[1]*math.Pi/180
	lat2, lon2 := b[0]*math.Pi/180, b[1]*math.Pi/180
	dlat := lat2 - lat1
	dlon := lon2 - lon1
	s := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)
	return R * 2 * math.Atan2(math.Sqrt(s), math.Sqrt(1-s))
}

// parseTrack extracts the route line from the C2C geometry geom_detail field.
// Returns WGS84 [lat, lon] pairs.
func parseTrack(m map[string]any) [][2]float64 {
	geomObj, ok := m["geometry"].(map[string]any)
	if !ok {
		return nil
	}
	detailStr, ok := geomObj["geom_detail"].(string)
	if !ok || detailStr == "" {
		return nil
	}
	var geojson struct {
		Coordinates [][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(detailStr), &geojson); err != nil {
		return nil
	}
	track := make([][2]float64, len(geojson.Coordinates))
	for i, c := range geojson.Coordinates {
		lat, lon := webMercatorToWGS84(c[0], c[1])
		track[i] = [2]float64{lat, lon}
	}
	return track
}
