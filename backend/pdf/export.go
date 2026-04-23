package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const exportTemplate = `<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="UTF-8">
<style>
  body { font-family: 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #fff; color: #1F2782; }
  h1 { font-size: 2em; color: #1F2782; border-bottom: 3px solid #1F2782; padding-bottom: 8px; }
  h2 { font-size: 1.2em; color: #1F2782; margin-top: 20px; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 20px; }
  .card { background: #f8f9ff; border: 1px solid #dde; border-radius: 8px; padding: 14px; }
  .badge { display: inline-block; background: #1F2782; color: #fff; border-radius: 4px; padding: 2px 8px; font-size: 0.85em; }
  table { width: 100%; border-collapse: collapse; }
  td, th { border: 1px solid #ccd; padding: 6px 10px; text-align: left; }
  th { background: #1F2782; color: #fff; }
  .risk-1 { color: #4caf50; } .risk-2 { color: #8bc34a; }
  .risk-3 { color: #ff9800; } .risk-4 { color: #f44336; } .risk-5 { color: #000; }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<div class="grid">
  <div class="card">
    <h2>Itinéraire</h2>
    <p><strong>Difficulté:</strong> <span class="badge">{{.Difficulty}}</span></p>
    <p><strong>Dénivelé:</strong> {{.ElevationGain}} m</p>
    <p><strong>Distance:</strong> {{printf "%.1f" .DistanceKm}} km</p>
    <p>{{.Description}}</p>
  </div>
  <div class="card">
    <h2>Météo</h2>
    <p>Min: {{printf "%.0f" .Weather.Forecast.TemperatureMin}}°C / Max: {{printf "%.0f" .Weather.Forecast.TemperatureMax}}°C</p>
    <p>Précipitations: {{printf "%.1f" .Weather.Forecast.Precipitation}} mm</p>
    <p>Vent: {{printf "%.0f" .Weather.Forecast.WindSpeedKmh}} km/h</p>
    <p>Avalanche: <span class="risk-{{.Weather.Avalanche.RiskLevel}}">{{.Weather.Avalanche.RiskLabel}}</span></p>
  </div>
</div>
<div class="grid" style="margin-top:20px">
  <div class="card">
    <h2>Horaire</h2>
    <p>Durée estimée: {{printf "%.1f" .Schedule.EstimatedDurationHours}} h</p>
    <p>Départ: {{.Schedule.RecommendedStartTime}} — Retour: {{.Schedule.RecommendedEndTime}}</p>
    {{if eq .Schedule.Source "formula"}}<p><em>Durée estimée par la règle de Naismith.</em></p>{{end}}
  </div>
  <div class="card">
    <h2>Matériel</h2>
    <table>
      <tr><th>Item</th><th>Qté</th><th>Notes</th></tr>
      {{range .Equipment}}<tr><td>{{.Item}}</td><td>{{.Quantity}}</td><td>{{.Notes}}</td></tr>{{end}}
    </table>
  </div>
</div>
<div class="card" style="margin-top:20px">
  <h2>Risques</h2>
  <ul>{{range .Risks}}<li>{{.}}</li>{{end}}</ul>
</div>
<p style="margin-top:30px;font-size:0.75em;color:#999">Généré le {{.GeneratedAt}} — mountain-race</p>
</body>
</html>`

// PlanData is the JSON body expected by the PDF export endpoint.
type PlanData struct {
	ID            string          `json:"id"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Difficulty    string          `json:"difficulty"`
	ElevationGain int             `json:"elevation_gain"`
	DistanceKm    float64         `json:"distance_km"`
	Equipment     []EquipmentData `json:"equipment"`
	Risks         []string        `json:"risks"`
	Schedule      ScheduleData    `json:"schedule"`
	Weather       WeatherData     `json:"weather"`
	GeneratedAt   string          `json:"-"`
}

type EquipmentData struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}

type ScheduleData struct {
	EstimatedDurationHours float64 `json:"estimated_duration_hours"`
	RecommendedStartTime   string  `json:"recommended_start_time"`
	RecommendedEndTime     string  `json:"recommended_end_time"`
	Source                 string  `json:"source"`
}

type WeatherData struct {
	Forecast  ForecastData  `json:"forecast"`
	Avalanche AvalancheData `json:"avalanche"`
}

type ForecastData struct {
	TemperatureMin float64 `json:"temperature_min_c"`
	TemperatureMax float64 `json:"temperature_max_c"`
	Precipitation  float64 `json:"precipitation_mm"`
	WindSpeedKmh   float64 `json:"wind_speed_kmh"`
	Condition      string  `json:"condition"`
}

type AvalancheData struct {
	RiskLevel   int    `json:"risk_level"`
	RiskLabel   string `json:"risk_label"`
	Description string `json:"description"`
}

// Generate renders the plan as a PDF and returns the bytes.
func Generate(_ http.ResponseWriter, body []byte) ([]byte, error) {
	var plan PlanData
	if err := json.Unmarshal(body, &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	plan.GeneratedAt = time.Now().Format("02/01/2006 15:04")

	tmpl, err := template.New("pdf").Parse(exportTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, plan); err != nil {
		return nil, err
	}
	htmlContent := buf.String()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dataURL := "data:text/html," + url.PathEscape(htmlContent)

	var pdfBuf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(dataURL),
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
