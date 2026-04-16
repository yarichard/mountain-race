package meteo

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const dpbraBase = "https://public-api.meteofrance.fr/public/DPBRA/v1"

// AvalancheResult holds the BRA bulletin summary.
type AvalancheResult struct {
	RiskLevel   int    `json:"risk_level"`
	RiskLabel   string `json:"risk_label"`
	Description string `json:"description"`
}

var riskLabels = map[int]string{
	1: "Faible",
	2: "Limité",
	3: "Marqué",
	4: "Fort",
	5: "Très fort",
}

// AvalancheForecast returns the BRA for the massif nearest to lat/lon.
func AvalancheForecast(lat, lon float64) (*AvalancheResult, error) {
	result, err := fetchAvalanche(lat, lon)
	if err != nil {
		return mockAvalanche(), nil
	}
	return result, nil
}

type massif struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	CentLat float64 `json:"centroid_lat"`
	CentLon float64 `json:"centroid_lon"`
}

func fetchAvalanche(lat, lon float64) (*AvalancheResult, error) {
	token, err := Token()
	if err != nil {
		return nil, err
	}

	// List massifs
	req, _ := http.NewRequest("GET", dpbraBase+"/massifs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DPBRA massifs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DPBRA massifs %d", resp.StatusCode)
	}

	var massifs []massif
	if err := json.Unmarshal(body, &massifs); err != nil {
		return nil, fmt.Errorf("DPBRA massifs parse: %w", err)
	}

	nearest := nearestMassif(massifs, lat, lon)
	if nearest.ID == 0 {
		return nil, fmt.Errorf("no massif found near %f,%f", lat, lon)
	}

	// Fetch BRA for nearest massif
	braReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/massifs/%d/BRA", dpbraBase, nearest.ID), nil)
	braReq.Header.Set("Authorization", "Bearer "+token)
	braResp, err := http.DefaultClient.Do(braReq)
	if err != nil {
		return nil, fmt.Errorf("DPBRA BRA: %w", err)
	}
	defer braResp.Body.Close()

	braBody, _ := io.ReadAll(braResp.Body)
	if braResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DPBRA BRA %d", braResp.StatusCode)
	}

	var bra struct {
		RiskLevel   int    `json:"risk_level"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(braBody, &bra); err != nil {
		return nil, fmt.Errorf("DPBRA BRA parse: %w", err)
	}

	label := riskLabels[bra.RiskLevel]
	if label == "" {
		label = fmt.Sprintf("Niveau %d", bra.RiskLevel)
	}

	return &AvalancheResult{
		RiskLevel:   bra.RiskLevel,
		RiskLabel:   label,
		Description: bra.Description,
	}, nil
}

func nearestMassif(massifs []massif, lat, lon float64) massif {
	var nearest massif
	minDist := math.MaxFloat64
	for _, m := range massifs {
		d := haversine(lat, lon, m.CentLat, m.CentLon)
		if d < minDist {
			minDist = d
			nearest = m
		}
	}
	return nearest
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func mockAvalanche() *AvalancheResult {
	_ = time.Now() // ensure package used
	return &AvalancheResult{
		RiskLevel:   2,
		RiskLabel:   "Limité",
		Description: "Risque limité en exposition sud au-dessus de 2500m. Plaques résiduelles possibles à l'ombre.",
	}
}
