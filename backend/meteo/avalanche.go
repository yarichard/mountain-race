package meteo

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const dpbraBase = "https://public-api.meteofrance.fr/public/DPBRA/v1"

// AvalancheResult holds the BRA bulletin summary.
type AvalancheResult struct {
	RiskLevel   int    `json:"risk_level"`
	RiskLabel   string `json:"risk_label"`
	Description string `json:"description"`
	MassifID    int    `json:"massif_id"`
	MassifName  string `json:"massif_name"`
}

var riskLabels = map[int]string{
	1: "Faible",
	2: "Limité",
	3: "Marqué",
	4: "Fort",
	5: "Très fort",
}

// AvalancheForecast returns the BRA for the massif nearest to lat/lon on date.
// Falls back to a mock result if the API is unavailable.
func AvalancheForecast(lat, lon float64, date time.Time) (*AvalancheResult, error) {
	return fetchAvalanche(lat, lon, date)
}

type massif struct {
	ID          int
	Name        string
	CentLat     float64
	CentLon     float64
	Coordinates [][][][]float64 // GeoJSON MultiPolygon: [polygon[ring[point[lon,lat]]]]
}

// massifFeatureCollection matches the GeoJSON shape returned by /liste-massifs.
type massifFeatureCollection struct {
	Features []struct {
		Properties struct {
			Code      int     `json:"code"`
			Title     string  `json:"title"`
			LatCenter float64 `json:"lat_center"`
			LonCenter float64 `json:"lon_center"`
		} `json:"properties"`
		Geometry struct {
			Coordinates [][][][]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

// braXML maps the RISQUES/RISQUE elements from the BRA XML response.
type braXML struct {
	Risques struct {
		Items []struct {
			Date       string `xml:"DATE,attr"`
			RisqueMaxi int    `xml:"RISQUEMAXI,attr"`
		} `xml:"RISQUE"`
	} `xml:"RISQUES"`
}

func fetchAvalanche(lat, lon float64, date time.Time) (*AvalancheResult, error) {
	token, err := Token()
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", dpbraBase+"/liste-massifs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DPBRA massifs: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DPBRA massifs %d", resp.StatusCode)
	}

	var fc massifFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		return nil, fmt.Errorf("DPBRA massifs parse: %w", err)
	}

	var massifs []massif
	for _, f := range fc.Features {
		massifs = append(massifs, massif{
			ID:          f.Properties.Code,
			Name:        f.Properties.Title,
			CentLat:     f.Properties.LatCenter,
			CentLon:     f.Properties.LonCenter,
			Coordinates: f.Geometry.Coordinates,
		})
	}

	nearest := nearestMassif(massifs, lat, lon)
	if nearest == nil {
		return nil, fmt.Errorf("position %f,%f is not inside any massif", lat, lon)
	}

	braReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/massif/BRA?id-massif=%d&format=xml", dpbraBase, nearest.ID), nil)
	braReq.Header.Set("Authorization", "Bearer "+token)
	braResp, err := httpClient.Do(braReq)
	if err != nil {
		return nil, fmt.Errorf("DPBRA BRA: %w", err)
	}
	defer braResp.Body.Close()

	braBody, _ := io.ReadAll(braResp.Body)
	if braResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DPBRA BRA %d", braResp.StatusCode)
	}

	var bra braXML
	if err := xml.Unmarshal(braBody, &bra); err != nil {
		return nil, fmt.Errorf("DPBRA BRA parse: %w", err)
	}

	targetDate := date.Format("2006-01-02")
	riskLevel := 0
	// Find the entry matching date; fall back to first entry.
	for _, r := range bra.Risques.Items {
		if strings.HasPrefix(r.Date, targetDate) {
			riskLevel = r.RisqueMaxi
			break
		}
	}
	if riskLevel == 0 && len(bra.Risques.Items) > 0 {
		riskLevel = bra.Risques.Items[0].RisqueMaxi
	}

	label := riskLabels[riskLevel]
	if label == "" {
		label = fmt.Sprintf("Niveau %d", riskLevel)
	}

	return &AvalancheResult{
		RiskLevel:  riskLevel,
		RiskLabel:  label,
		MassifID:   nearest.ID,
		MassifName: nearest.Name,
	}, nil
}

// nearestMassif returns the massif whose polygon contains lat/lon with the
// smallest centroid distance, or nil if the point is not inside any massif.
func nearestMassif(massifs []massif, lat, lon float64) *massif {
	var nearest *massif
	minDist := math.MaxFloat64
	for i := range massifs {
		m := &massifs[i]
		if !pointInMultiPolygon(m.Coordinates, lat, lon) {
			continue
		}
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

// ProxyMassifImage fetches a massif image from DPBRA and writes it to w.
func ProxyMassifImage(w io.Writer, massifID int, imageType string) (string, error) {
	token, err := Token()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/massif/image/%s?id-massif=%d", dpbraBase, imageType, massifID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DPBRA image %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	io.Copy(w, resp.Body)
	return ct, nil
}

func mockAvalanche() *AvalancheResult {
	return &AvalancheResult{
		RiskLevel:   2,
		RiskLabel:   "Limité",
		Description: "Risque limité en exposition sud au-dessus de 2500m. Plaques résiduelles possibles à l'ombre.",
		MassifID:    0,
		MassifName:  "",
	}
}
