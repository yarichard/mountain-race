package pdf

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// c2cImageBase is a var so tests can override it with an httptest.Server URL.
var (
	c2cImageBase = "https://media.camptocamp.org/c2corg-active/"
	maxImages    = 4
)

// ── Data structs ──────────────────────────────────────────────────────────────

// ParticipantData holds one participant's info.
type ParticipantData struct {
	Name          string `json:"name"`
	ClimbingLevel string `json:"climbingLevel"`
}

// AlternativeRoute is a related fallback route.
type AlternativeRoute struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// EquipmentData is one item in the gear list.
type EquipmentData struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}

// ScheduleData holds timing estimates.
type ScheduleData struct {
	EstimatedDurationHours float64 `json:"estimated_duration_hours"`
	RecommendedStartTime   string  `json:"recommended_start_time"`
	RecommendedEndTime     string  `json:"recommended_end_time"`
	Source                 string  `json:"source"` // "camptocamp" | "formula"
}

// WeatherData aggregates forecast and avalanche risk.
type WeatherData struct {
	Forecast  ForecastData  `json:"forecast"`
	Avalanche AvalancheData `json:"avalanche"`
}

// ForecastData is the Open-Meteo daily summary.
type ForecastData struct {
	TemperatureMin float64 `json:"temperature_min_c"`
	TemperatureMax float64 `json:"temperature_max_c"`
	Precipitation  float64 `json:"precipitation_mm"`
	WindSpeedKmh   float64 `json:"wind_speed_kmh"`
	Condition      string  `json:"condition"`
}

// AvalancheData is the DPBRA risk summary.
type AvalancheData struct {
	RiskLevel   int    `json:"risk_level"`
	RiskLabel   string `json:"risk_label"`
	Description string `json:"description"`
}

// PlanData is the JSON body expected by POST /api/export/pdf.
type PlanData struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	Difficulty        string             `json:"difficulty"`
	ElevationGain     int                `json:"elevation_gain"`
	HeightDiffDown    int                `json:"height_diff_down"`
	DistanceKm        float64            `json:"distance_km"`
	Lat               float64            `json:"lat"`
	Lon               float64            `json:"lon"`
	Track             [][2]float64       `json:"track,omitempty"`
	ElevationProfile  [][2]float64       `json:"elevation_profile,omitempty"`
	Images            []string           `json:"images,omitempty"`
	Equipment         []EquipmentData    `json:"equipment"`
	Risks             []string           `json:"risks"`
	AlternativeRoutes []AlternativeRoute `json:"alternative_routes"`
	Schedule          ScheduleData       `json:"schedule"`
	Weather           WeatherData        `json:"weather"`
	Participants      []ParticipantData  `json:"participants"`
	Objectives        []string           `json:"objectives"`
	Notes             string             `json:"notes"`
	// Computed (not from JSON)
	GeneratedAt     string        `json:"-"`
	DescriptionHTML template.HTML `json:"-"`
	ImagesHTML      template.HTML `json:"-"`
	ElevationSVG    template.HTML `json:"-"`
	MapSVG          template.HTML `json:"-"`
}

// ── Markdown helper ───────────────────────────────────────────────────────────

var (
	reBold = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reH3   = regexp.MustCompile(`(?m)^###\s*(.+)$`)
	reH2   = regexp.MustCompile(`(?m)^##\s*(.+)$`)
)

// markdownToHTML converts a simple subset of Markdown to safe HTML.
// It escapes the raw text first, then applies transformations.
func markdownToHTML(text string) template.HTML {
	safe := html.EscapeString(text)
	safe = reH3.ReplaceAllString(safe, "<h4>$1</h4>")
	safe = reH2.ReplaceAllString(safe, "<h3>$1</h3>")
	safe = reBold.ReplaceAllString(safe, "<strong>$1</strong>")
	safe = strings.ReplaceAll(safe, "\n\n", "</p><p>")
	safe = strings.ReplaceAll(safe, "\n", "<br>")
	return template.HTML("<p>" + safe + "</p>")
}

// ── Image fetching ────────────────────────────────────────────────────────────

var imageClient = &http.Client{Timeout: 5 * time.Second}

func fetchImage(filename string) (string, error) {
	resp, err := imageClient.Get(c2cImageBase + filename)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// buildImagesHTML fetches up to maxImages images in parallel and returns
// a pre-built HTML fragment of <img> tags with base64-encoded data URIs.
func buildImagesHTML(filenames []string) template.HTML {
	if len(filenames) == 0 {
		return ""
	}
	if len(filenames) > maxImages {
		filenames = filenames[:maxImages]
	}
	results := make([]string, len(filenames))
	var wg sync.WaitGroup
	for i, fn := range filenames {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			if dataURL, err := fetchImage(name); err == nil {
				results[idx] = dataURL
			}
		}(i, fn)
	}
	wg.Wait()

	var buf strings.Builder
	for _, r := range results {
		if r != "" {
			fmt.Fprintf(&buf, `<img src="%s" alt="Photo du parcours">`, r)
		}
	}
	return template.HTML(buf.String())
}

// ── SVG elevation profile ─────────────────────────────────────────────────────

func buildElevationSVG(profile [][2]float64) template.HTML {
	if len(profile) < 2 {
		return ""
	}

	const (
		svgW   = 400.0
		svgH   = 110.0
		padL   = 42.0
		padR   = 10.0
		padT   = 8.0
		padB   = 22.0
		chartW = svgW - padL - padR
		chartH = svgH - padT - padB
	)

	minDist := profile[0][0]
	maxDist := profile[len(profile)-1][0]
	minElev := profile[0][1]
	maxElev := profile[0][1]
	for _, p := range profile {
		if p[1] < minElev {
			minElev = p[1]
		}
		if p[1] > maxElev {
			maxElev = p[1]
		}
	}

	elevRange := maxElev - minElev
	if elevRange < 1 {
		elevRange = 1
	}
	distRange := maxDist - minDist
	if distRange < 0.001 {
		distRange = 0.001
	}

	toXY := func(dist, elev float64) (float64, float64) {
		x := padL + (dist-minDist)/distRange*chartW
		y := padT + (1-(elev-minElev)/elevRange)*chartH
		return x, y
	}

	// Polyline points
	var linePts strings.Builder
	for _, p := range profile {
		x, y := toXY(p[0], p[1])
		fmt.Fprintf(&linePts, "%.1f,%.1f ", x, y)
	}

	// Fill polygon (close at the bottom)
	x0, _ := toXY(profile[0][0], profile[0][1])
	xN, _ := toXY(profile[len(profile)-1][0], profile[len(profile)-1][1])
	bottomY := padT + chartH
	fillPts := fmt.Sprintf("%.1f,%.1f %s%.1f,%.1f", x0, bottomY, linePts.String(), xN, bottomY)

	// Y-axis labels
	midElev := (minElev + maxElev) / 2.0

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" style="width:100%%;height:110px">`, svgW, svgH)
	// Background
	fmt.Fprintf(&sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#f5f7ff" rx="2"/>`, padL, padT, chartW, chartH)
	// Fill
	fmt.Fprintf(&sb, `<polygon points="%s" fill="#b8c4e8" opacity="0.6"/>`, fillPts)
	// Line
	fmt.Fprintf(&sb, `<polyline points="%s" fill="none" stroke="#1F2782" stroke-width="1.5" stroke-linejoin="round"/>`, linePts.String())
	// Axis lines
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#aab" stroke-width="0.5"/>`, padL, padT, padL, bottomY)
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#aab" stroke-width="0.5"/>`, padL, bottomY, padL+chartW, bottomY)
	// Y-axis labels
	fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="7" fill="#555" text-anchor="end">%.0fm</text>`, padL-2, padT+5, maxElev)
	fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="7" fill="#555" text-anchor="end">%.0fm</text>`, padL-2, padT+chartH/2+3, midElev)
	fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="7" fill="#555" text-anchor="end">%.0fm</text>`, padL-2, bottomY, minElev)
	// X-axis distance label
	fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="7" fill="#555">%.1fkm</text>`, padL+chartW-10, bottomY+12, maxDist)
	fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="7" fill="#555">0km</text>`, padL, bottomY+12)
	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
}

// ── SVG map ───────────────────────────────────────────────────────────────────

func buildMapSVG(track [][2]float64, lat, lon float64) template.HTML {
	var points [][2]float64
	if len(track) >= 2 {
		points = track
	} else if lat != 0 || lon != 0 {
		points = [][2]float64{{lat, lon}}
	} else {
		return ""
	}

	const (
		svgW   = 300.0
		svgH   = 180.0
		padAll = 14.0
	)

	// Bounding box
	minLat, maxLat := points[0][0], points[0][0]
	minLon, maxLon := points[0][1], points[0][1]
	for _, p := range points {
		if p[0] < minLat {
			minLat = p[0]
		}
		if p[0] > maxLat {
			maxLat = p[0]
		}
		if p[1] < minLon {
			minLon = p[1]
		}
		if p[1] > maxLon {
			maxLon = p[1]
		}
	}

	// Padding proportional to bbox (min 0.003° ≈ 300m)
	latPad := math.Max((maxLat-minLat)*0.18, 0.003)
	lonPad := math.Max((maxLon-minLon)*0.18, 0.003)
	minLat -= latPad
	maxLat += latPad
	minLon -= lonPad
	maxLon += lonPad

	chartW := svgW - 2*padAll
	chartH := svgH - 2*padAll
	latRange := maxLat - minLat
	lonRange := maxLon - minLon

	toXY := func(pt [2]float64) (float64, float64) {
		x := padAll + (pt[1]-minLon)/lonRange*chartW
		y := padAll + (1-(pt[0]-minLat)/latRange)*chartH
		return x, y
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" style="width:100%%;height:180px;border-radius:4px">`, svgW, svgH)
	// Background
	sb.WriteString(fmt.Sprintf(`<rect width="%.0f" height="%.0f" fill="#dce8f5" rx="4"/>`, svgW, svgH))

	// Grid lines
	for i := 0; i <= 3; i++ {
		gy := padAll + float64(i)*chartH/3
		gx := padAll + float64(i)*chartW/3
		fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#c0d4e8" stroke-width="0.5"/>`, padAll, gy, padAll+chartW, gy)
		fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#c0d4e8" stroke-width="0.5"/>`, gx, padAll, gx, padAll+chartH)
	}

	if len(points) >= 2 {
		// Track shadow
		var ptsStr strings.Builder
		for _, p := range points {
			x, y := toXY(p)
			fmt.Fprintf(&ptsStr, "%.1f,%.1f ", x, y)
		}
		fmt.Fprintf(&sb, `<polyline points="%s" fill="none" stroke="rgba(0,0,0,0.2)" stroke-width="4" stroke-linejoin="round" stroke-linecap="round"/>`, ptsStr.String())
		// Track
		fmt.Fprintf(&sb, `<polyline points="%s" fill="none" stroke="#1F2782" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round"/>`, ptsStr.String())
		// Start marker
		sx, sy := toXY(points[0])
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="4.5" fill="white" stroke="#1F2782" stroke-width="1.5"/>`, sx, sy)
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="2.5" fill="#4caf50"/>`, sx, sy)
		// End marker
		ex, ey := toXY(points[len(points)-1])
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="4.5" fill="white" stroke="#1F2782" stroke-width="1.5"/>`, ex, ey)
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="2.5" fill="#f44336"/>`, ex, ey)
	} else {
		// Single point marker (pin)
		x, y := toXY(points[0])
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="7" fill="#1F2782" stroke="white" stroke-width="2"/>`, x, y)
		fmt.Fprintf(&sb, `<circle cx="%.1f" cy="%.1f" r="3" fill="white"/>`, x, y)
	}

	sb.WriteString(`</svg>`)
	return template.HTML(sb.String())
}

// ── HTML template ─────────────────────────────────────────────────────────────

const exportTemplate = `<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="UTF-8">
<style>
  @page { size: A4 landscape; }
  * { box-sizing: border-box; }
  body {
    font-family: 'Segoe UI', Tahoma, Geneva, sans-serif;
    font-size: 8pt;
    margin: 0; padding: 10px 12px;
    background: #fff; color: #1F2782;
    line-height: 1.35;
  }
  h1 { font-size: 14pt; color: #1F2782; border-bottom: 3px solid #1F2782; padding-bottom: 3px; margin: 0 0 6px 0; }
  h2 { font-size: 8.5pt; color: #1F2782; margin: 0 0 4px 0; border-bottom: 1px solid #dde; padding-bottom: 2px; }
  h3 { font-size: 8pt; color: #1F2782; margin: 5px 0 2px 0; }
  h4 { font-size: 7.5pt; color: #444; margin: 3px 0 1px 0; }
  p  { margin: 0 0 3px 0; }
  ul { margin: 2px 0; padding-left: 14px; }
  li { margin-bottom: 1px; }
  .badge {
    display: inline-block;
    background: #1F2782; color: #fff;
    border-radius: 4px; padding: 2px 8px;
    font-size: 9pt; font-weight: bold;
  }
  .meta { display: flex; gap: 12px; align-items: center; margin-bottom: 8px; flex-wrap: wrap; }
  .meta-item { font-size: 8pt; color: #444; }
  .row { display: flex; gap: 8px; margin-bottom: 8px; }
  .col  { flex: 1; }
  .col2 { flex: 2; }
  .col3 { flex: 3; }
  .card {
    background: #f5f7ff;
    border: 1px solid #dde;
    border-radius: 6px;
    padding: 7px 9px;
    height: 100%;
  }
  .description { max-height: 155px; overflow: hidden; font-size: 7.5pt; }
  table { width: 100%; border-collapse: collapse; font-size: 7.5pt; }
  td, th { border: 1px solid #ccd; padding: 3px 5px; text-align: left; vertical-align: top; }
  th { background: #1F2782; color: #fff; }
  tr:nth-child(even) td { background: #eef1fd; }
  .risk-0 { color: #aaa; }
  .risk-1 { color: #4caf50; font-weight: bold; }
  .risk-2 { color: #8bc34a; font-weight: bold; }
  .risk-3 { color: #ff9800; font-weight: bold; }
  .risk-4 { color: #f44336; font-weight: bold; }
  .risk-5 { background: #f44336; color: #fff; font-weight: bold; padding: 1px 4px; border-radius: 3px; }
  .photos { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 4px; }
  .photos img { height: 90px; width: auto; max-width: 170px; border-radius: 4px; object-fit: cover; }
  .tag {
    display: inline-block;
    background: #e8ecff; color: #1F2782;
    border-radius: 10px; padding: 1px 7px;
    font-size: 7pt; margin: 1px 2px;
    border: 1px solid #ccd;
  }
  .footer { margin-top: 8px; font-size: 6.5pt; color: #aaa; text-align: right; }
  .naismith { font-size: 7pt; color: #888; margin-top: 3px; font-style: italic; }
  .participants-header { display: flex; justify-content: space-between; align-items: flex-start; }
  .participants-list { text-align: right; font-size: 7.5pt; }
</style>
</head>
<body>

<!-- ── Header ── -->
<div class="participants-header">
  <div>
    <h1>{{.Title}}</h1>
    <div class="meta">
      {{if .Difficulty}}<span class="badge">{{.Difficulty}}</span>{{end}}
      <span class="meta-item">↑ {{.ElevationGain}} m{{if .HeightDiffDown}}&nbsp;&nbsp;↓ {{.HeightDiffDown}} m{{end}}</span>
      {{if .DistanceKm}}<span class="meta-item">{{printf "%.1f" .DistanceKm}} km</span>{{end}}
    </div>
  </div>
  {{if .Participants}}
  <div class="participants-list">
    <strong style="font-size:7pt;color:#888">PARTICIPANTS</strong><br>
    {{range .Participants}}<div><strong>{{.Name}}</strong>{{if .ClimbingLevel}}&nbsp;<span style="color:#666">({{.ClimbingLevel}})</span>{{end}}</div>{{end}}
  </div>
  {{end}}
</div>

<!-- ── Row 1: Description + Weather/Schedule ── -->
<div class="row">
  <div class="col2">
    <div class="card">
      <h2>Itinéraire</h2>
      <div class="description">{{.DescriptionHTML}}</div>
    </div>
  </div>
  <div class="col">
    <div class="card" style="margin-bottom:8px">
      <h2>Météo &amp; Avalanche</h2>
      <div style="display:flex;gap:12px">
        <div>
          <div>🌡 {{printf "%.0f" .Weather.Forecast.TemperatureMin}}°C / {{printf "%.0f" .Weather.Forecast.TemperatureMax}}°C</div>
          <div>🌧 {{printf "%.1f" .Weather.Forecast.Precipitation}} mm</div>
          <div>💨 {{printf "%.0f" .Weather.Forecast.WindSpeedKmh}} km/h</div>
        </div>
        <div>
          <div style="font-size:7pt;color:#888">Risque avalanche</div>
          <div class="risk-{{.Weather.Avalanche.RiskLevel}}">{{if .Weather.Avalanche.RiskLabel}}{{.Weather.Avalanche.RiskLabel}}{{else}}—{{end}}</div>
          {{if .Weather.Avalanche.Description}}<div style="font-size:6.5pt;color:#555;margin-top:2px">{{.Weather.Avalanche.Description}}</div>{{end}}
        </div>
      </div>
    </div>
    <div class="card">
      <h2>Horaire</h2>
      <div>Durée: <strong>{{printf "%.1f" .Schedule.EstimatedDurationHours}} h</strong></div>
      <div>Départ <strong>{{.Schedule.RecommendedStartTime}}</strong> — Retour <strong>{{.Schedule.RecommendedEndTime}}</strong></div>
      {{if eq .Schedule.Source "formula"}}<div class="naismith">⚠ Estimé par la règle de Naismith</div>{{end}}
    </div>
  </div>
</div>

<!-- ── Row 2: Elevation profile + Map (only when data available) ── -->
{{if or .ElevationSVG .MapSVG}}
<div class="row">
  {{if .ElevationSVG}}
  <div class="col2">
    <div class="card">
      <h2>Profil altimétrique</h2>
      {{.ElevationSVG}}
    </div>
  </div>
  {{end}}
  {{if .MapSVG}}
  <div class="col">
    <div class="card">
      <h2>Carte</h2>
      {{.MapSVG}}
    </div>
  </div>
  {{end}}
</div>
{{end}}

<!-- ── Photos (only when images available) ── -->
{{if .ImagesHTML}}
<div class="card" style="margin-bottom:8px">
  <h2>Photos</h2>
  <div class="photos">{{.ImagesHTML}}</div>
</div>
{{end}}

<!-- ── Row 3: Equipment + Risks/Objectives + Alternatives ── -->
<div class="row">
  <div class="col">
    <div class="card">
      <h2>Matériel</h2>
      {{if .Equipment}}
      <table>
        <tr><th>Item</th><th>Qté</th><th>Notes</th></tr>
        {{range .Equipment}}<tr><td>{{.Item}}</td><td>{{.Quantity}}</td><td>{{.Notes}}</td></tr>{{end}}
      </table>
      {{else}}<div style="color:#999;font-size:7.5pt">Aucun matériel spécifié</div>{{end}}
    </div>
  </div>

  <div class="col">
    <div class="card">
      <h2>Risques &amp; Vigilance</h2>
      {{if .Risks}}<ul>{{range .Risks}}<li>{{.}}</li>{{end}}</ul>{{else}}<div style="color:#999">—</div>{{end}}
      {{if .Objectives}}
      <h2 style="margin-top:6px">Objectifs</h2>
      <div>{{range .Objectives}}<span class="tag">{{.}}</span>{{end}}</div>
      {{end}}
      {{if .Notes}}<div style="margin-top:4px;font-size:7.5pt;color:#555">{{.Notes}}</div>{{end}}
    </div>
  </div>

  <div class="col">
    <div class="card">
      <h2>Itinéraires alternatifs</h2>
      {{if .AlternativeRoutes}}
      <ul>
        {{range .AlternativeRoutes}}
        <li><strong>{{.Title}}</strong>{{if .Reason}}<br><span style="font-size:7pt;color:#666">{{.Reason}}</span>{{end}}</li>
        {{end}}
      </ul>
      {{else}}<div style="color:#999;font-size:7.5pt">Aucun itinéraire alternatif</div>{{end}}
    </div>
  </div>
</div>

<div class="footer">Généré le {{.GeneratedAt}} — mountain-race</div>
</body>
</html>`

// Generate renders the plan as a PDF and returns the bytes.
func Generate(_ http.ResponseWriter, body []byte) ([]byte, error) {
	var plan PlanData
	if err := json.Unmarshal(body, &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	plan.GeneratedAt = time.Now().Format("02/01/2006 15:04")
	plan.DescriptionHTML = markdownToHTML(plan.Description)
	plan.ElevationSVG = buildElevationSVG(plan.ElevationProfile)
	plan.MapSVG = buildMapSVG(plan.Track, plan.Lat, plan.Lon)

	// Fetch images in parallel before starting Chromium to stay within the 30s timeout.
	plan.ImagesHTML = buildImagesHTML(plan.Images)

	tmpl, err := template.New("pdf").Parse(exportTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, plan); err != nil {
		return nil, err
	}
	htmlContent := buf.String()

	// Write HTML to a temp file so Chromium can load it via file:// URL.
	// This avoids the ~2MB data: URL size limit that base64-embedded images would exceed.
	tmpFile, err := os.CreateTemp("", "mountain-race-*.html")
	if err != nil {
		return nil, fmt.Errorf("create temp html: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(htmlContent); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp html: %w", err)
	}
	tmpFile.Close()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	fileURL := "file://" + tmpFile.Name()

	var pdfBuf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(fileURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithLandscape(true).
				WithPrintBackground(true).
				WithPaperWidth(11.69).
				WithPaperHeight(8.27).
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("chromedp PDF: %w", err)
	}

	return pdfBuf, nil
}
