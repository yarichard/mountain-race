package camptocamp

import (
	"encoding/json"
	"fmt"
	"math"
)

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
	DistanceKm        float64            `json:"distance_km"`
	Lat               float64            `json:"lat"`
	Lon               float64            `json:"lon"`
	Pitches           []Pitch            `json:"pitches,omitempty"`
	TopoURL           string             `json:"topo_url"`
	GpxURL            string             `json:"gpx_url"`
	Equipment         []Equipment        `json:"equipment"`
	Risks             []string           `json:"risks"`
	AlternativeRoutes []AlternativeRoute `json:"alternative_routes"`
	Schedule          Schedule           `json:"schedule"`
	SourceURL         string             `json:"source_url"`
}

// GetDetail fetches full route detail from CampToCamp.
func GetDetail(id, lang string) (*RouteDetail, error) {
	data, err := get("/routes/" + id)
	if err != nil {
		return nil, err
	}

	locs := localesField(data)
	title := pickLocale(locs, lang, "title_prefix") + " / " + pickLocale(locs, lang, "title")
	description := pickLocale(locs, lang, "route_history") + "\n"
	description += pickLocale(locs, lang, "summary")+ "\n"
	description += pickLocale(locs, lang, "description")+ "\n"
	if description == "" {
		description = pickLocale(locs, lang, "route_history")+ "\n"
	}
	description += pickLocale(locs, lang, "external_resources")+ "\n"

	difficulty := bestGrade(data)
	elevGain := intField(data, "elevation_gain_up")
	routeLen := floatField(data, "route_length") / 1000

	pitches := parsePitches(data, lang)
	equipment := parseEquipment(data, lang)
	risks := parseRisks(data, lang)
	alts := parseAlternatives(data, lang)
	lat, lon := parseLatLon(data)

	// Schedule: check if C2C has duration data in comments/description
	sched := parseSchedule(data, routeLen, float64(elevGain))

	// Topo and GPX
	topoURL := firstImageURL(data)
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
		DistanceKm:        routeLen,
		Lat:               lat,
		Lon:               lon,
		Pitches:           pitches,
		TopoURL:           topoURL,
		GpxURL:            gpxURL,
		Equipment:         equipment,
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

func parseEquipment(m map[string]any, lang string) []Equipment {
	var result []Equipment

	if gearText := pickLocale(localesField(m), lang, "gear"); gearText != "" {
		result = append(result, Equipment{Item: gearText, Quantity: 1, Notes: ""})
	}

	// equipment_rating
	if er := stringField(m, "equipment_rating"); er != "" {
		result = append(result, Equipment{Item: "Équipement", Quantity: 1, Notes: er})
	}

	if len(result) == 0 {
		result = []Equipment{{Item: "Matériel standard", Quantity: 1, Notes: "Voir description de la voie"}}
	}
	return result
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

func parseSchedule(m map[string]any, distanceKm, elevGainM float64) Schedule {
	// Look for duration in locales
	locales, _ := m["locales"].([]any)
	for _, l := range locales {
		lm, ok := l.(map[string]any)
		if !ok {
			continue
		}
		// C2C sometimes has time_required field
		if d, ok := lm["time_required"].(string); ok && d != "" {
			return Schedule{
				EstimatedDurationHours: 6,
				RecommendedStartTime:   "06:00",
				RecommendedEndTime:     "16:00",
				Source:                 "camptocamp",
			}
		}
	}

	// Naismith fallback
	duration := (distanceKm / 5.0) + (elevGainM / 600.0)
	if duration < 1 {
		duration = 4
	}
	endHour := 6 + int(duration)
	if endHour > 20 {
		endHour = 20
	}

	return Schedule{
		EstimatedDurationHours: duration,
		RecommendedStartTime:   "06:00",
		RecommendedEndTime:     fmt.Sprintf("%02d:00", endHour),
		Source:                 "formula",
	}
}

func firstImageURL(m map[string]any) string {
	imgs, ok := m["images"].([]any)
	if !ok {
		return ""
	}
	for _, img := range imgs {
		im, ok := img.(map[string]any)
		if !ok {
			continue
		}
		if fn, ok := im["filename"].(string); ok && fn != "" {
			return "https://media.camptocamp.org/c2corg_active/" + fn
		}
	}
	return ""
}

// parseLatLon extracts WGS84 coordinates from the C2C geometry field.
// C2C stores geometry as a stringified GeoJSON Point in EPSG:3857 (Web Mercator).
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
	x, y := geojson.Coordinates[0], geojson.Coordinates[1]
	const R = 6378137.0
	lon = x / R * (180.0 / math.Pi)
	lat = (2*math.Atan(math.Exp(y/R)) - math.Pi/2) * (180.0 / math.Pi)
	return lat, lon
}
