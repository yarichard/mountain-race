package meteo

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ForecastResult holds the decoded weather forecast.
type ForecastResult struct {
	Date           string  `json:"date"`
	TemperatureMin float64 `json:"temperature_min_c"`
	TemperatureMax float64 `json:"temperature_max_c"`
	Precipitation  float64 `json:"precipitation_mm"`
	WindSpeedKmh   float64 `json:"wind_speed_kmh"`
	Condition      string  `json:"condition"`
}

// GRIB2 WMO parameter codes used by MeteoFrance.
const (
	catTemperature   = 0 // Discipline 0, Category 0
	catMoisture      = 1 // Discipline 0, Category 1
	catMomentum      = 2 // Discipline 0, Category 2
	paramTemperature = 0 // Temperature (K)
	paramPrecip      = 8 // Total precipitation (kg/m² = mm)
	paramWindSpeed   = 1 // Wind speed (m/s)

	bboxDelta = 0.15 // degrees; tight bounding box around target point
)

type wcsModel struct {
	baseURL string
	wcsPath string
}

var (
	aromeModel = wcsModel{
		baseURL: "https://public-api.meteofrance.fr/public/arome/1.0",
		wcsPath: "MF-NWP-HIGHRES-AROME-001-FRANCE-WCS",
	}
	arpegeModel = wcsModel{
		baseURL: "https://public-api.meteofrance.fr/public/arpege/1.0",
		wcsPath: "MF-NWP-GLOBAL-ARPEGE-025-GLOBE-WCS",
	}
)

// capsCache holds WCS coverage IDs, refreshed hourly per model.
var capsCache = struct {
	mu        sync.Mutex
	coverages map[string][]string
	fetchedAt map[string]time.Time
}{
	coverages: make(map[string][]string),
	fetchedAt: make(map[string]time.Time),
}

// Forecast returns the weather forecast for the given location and date.
// It uses AROME (≤48 h) or ARPEGE (>48 h) from MeteoFrance.
func Forecast(lat, lon float64, date time.Time) (*ForecastResult, error) {
	return fetchForecast(lat, lon, date)
}

func fetchForecast(lat, lon float64, date time.Time) (*ForecastResult, error) {
	token, err := Token()
	if err != nil {
		return nil, fmt.Errorf("meteo token: %w", err)
	}

	// Request morning and midday temperature, midday precipitation and wind.
	morn := time.Date(date.Year(), date.Month(), date.Day(), 6, 0, 0, 0, time.UTC)
	noon := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)

	// Use ARPEGE when the race day is 48 h or more after today's midnight (UTC).
	// Both sides are truncated to midnight so the comparison is stable regardless of current time.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	raceDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	model := aromeModel
	if raceDay.Sub(today) >= 48*time.Hour {
		model = arpegeModel
	}

	coverages, err := fetchCoverageIDs(token, model)
	if err != nil {
		return nil, fmt.Errorf("WCS capabilities: %w", err)
	}

	tempMin, err := fetchParam(token, model, coverages, "TEMPERATURE", lat, lon, morn, 2, catTemperature, paramTemperature)
	if err != nil {
		return nil, fmt.Errorf("temperature (min): %w", err)
	}
	tempMax, err := fetchParam(token, model, coverages, "TEMPERATURE", lat, lon, noon, 2, catTemperature, paramTemperature)
	if err != nil {
		return nil, fmt.Errorf("temperature (max): %w", err)
	}

	// Precipitation and wind are best-effort; non-fatal on failure.
	precip, _ := fetchParam(token, model, coverages, "TOTAL_PRECIPITATION", lat, lon, noon, 0, catMoisture, paramPrecip)
	wind, _ := fetchParam(token, model, coverages, "WIND_SPEED", lat, lon, noon, 10, catMomentum, paramWindSpeed)

	tempMinC := tempMin - 273.15
	tempMaxC := tempMax - 273.15
	windKmh := wind * 3.6

	return &ForecastResult{
		Date:           date.Format("2006-01-02"),
		TemperatureMin: math.Round(tempMinC*10) / 10,
		TemperatureMax: math.Round(tempMaxC*10) / 10,
		Precipitation:  math.Round(precip*10) / 10,
		WindSpeedKmh:   math.Round(windKmh),
		Condition:      deriveCondition(tempMaxC, precip, windKmh),
	}, nil
}

// fetchCoverageIDs fetches and caches WCS coverage IDs for a given model.
func fetchCoverageIDs(token string, model wcsModel) ([]string, error) {
	capsCache.mu.Lock()
	defer capsCache.mu.Unlock()

	if ids, ok := capsCache.coverages[model.wcsPath]; ok {
		if time.Since(capsCache.fetchedAt[model.wcsPath]) < time.Hour {
			return ids, nil
		}
	}

	url := fmt.Sprintf("%s/wcs/%s/GetCapabilities?SERVICE=WCS&VERSION=2.0.1&REQUEST=GetCapabilities&language=eng",
		model.baseURL, model.wcsPath)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("GetCapabilities HTTP %d: %s", resp.StatusCode, body)
	}

	// Tee response into a temp file for debugging.
	tmpFile, _ := os.CreateTemp("/workspaces/mountain-race", "wcs_capabilities_*.xml")
	body := io.TeeReader(resp.Body, tmpFile)
	ids, err := parseCoverageIDs(body)
	tmpFile.Close()
	if err != nil {
		return nil, err
	}

	capsCache.coverages[model.wcsPath] = ids
	capsCache.fetchedAt[model.wcsPath] = time.Now()
	return ids, nil
}

// parseCoverageIDs streams the WCS GetCapabilities XML and extracts coverage IDs.
func parseCoverageIDs(r io.Reader) ([]string, error) {
	var ids []string
	dec := xml.NewDecoder(r)
	inCovID := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // return what we have
		}
		switch v := tok.(type) {
		case xml.StartElement:
			inCovID = v.Name.Local == "CoverageId"
		case xml.CharData:
			if inCovID {
				id := strings.TrimSpace(string(v))
				if id != "" {
					ids = append(ids, id)
				}
			}
		case xml.EndElement:
			if v.Name.Local == "CoverageId" {
				inCovID = false
			}
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no coverage IDs found in WCS GetCapabilities")
	}
	return ids, nil
}

// selectCoverageID finds the coverage whose valid-time timestamp (encoded in the ID as
// PARAM__LEVEL___2006-01-02T15.04.05Z) is closest to targetTime.
// No hard cutoff: the capabilities only contain past data, so we always pick the most
// recent available coverage as the best proxy for the requested forecast time.
// Each MeteoFrance WCS coverage represents exactly one valid time; subset=time() in
// GetCoverage must therefore match the coverage's own timestamp, not the caller's target.
func selectCoverageID(coverages []string, paramPrefix string, targetTime time.Time) (string, time.Time, error) {
	var bestID string
	var bestTime time.Time
	bestDiff := time.Duration(math.MaxInt64)

	for _, id := range coverages {
		if !strings.HasPrefix(id, paramPrefix) {
			continue
		}
		// ID format: PARAM__LEVEL___2006-01-02T15.04.05Z
		parts := strings.Split(id, "___")
		if len(parts) < 2 {
			continue
		}
		t, err := time.Parse("2006-01-02T15.04.05Z", parts[len(parts)-1])
		if err != nil {
			continue
		}
		diff := t.Sub(targetTime)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestTime = t
			bestID = id
		}
	}

	if bestID == "" {
		return "", time.Time{}, fmt.Errorf("no %s coverage found for target time %s", paramPrefix, targetTime.Format(time.RFC3339))
	}
	return bestID, bestTime, nil
}

// fetchParam downloads a GRIB2 GetCoverage response and returns the decoded scalar value.
// height=0 means no height subset (e.g. for surface precipitation).
func fetchParam(token string, model wcsModel, coverages []string,
	paramPrefix string, lat, lon float64, targetTime time.Time,
	height, category, paramNum int) (float64, error) {

	coverageID, coverageTime, err := selectCoverageID(coverages, paramPrefix, targetTime)
	if err != nil {
		return 0, err
	}

	// subset=time() must match the coverage's own valid timestamp exactly.
	subsets := fmt.Sprintf("&subset=lat(%.4f,%.4f)&subset=long(%.4f,%.4f)&subset=time(%s)",
		lat-bboxDelta, lat+bboxDelta,
		lon-bboxDelta, lon+bboxDelta,
		coverageTime.UTC().Format("2006-01-02T15:04:05Z"))
	// Surface-level coverages (GROUND_OR_WATER_SURFACE) do not accept a height subset.
	if height > 0 && !strings.Contains(coverageID, "SURFACE") {
		subsets += fmt.Sprintf("&subset=height(%d)", height)
	}

	url := fmt.Sprintf("%s/wcs/%s/GetCoverage?SERVICE=WCS&VERSION=2.0.1&REQUEST=GetCoverage&format=application%%2Fwmo-grib&coverageId=%s%s",
		model.baseURL, model.wcsPath, coverageID, subsets)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("GetCoverage HTTP %d for %s: %s", resp.StatusCode, coverageID, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return decodeGrib2Value(body, category, paramNum)
}

// decodeGrib2Value parses a GRIB2 binary stream and returns the mean of valid grid values
// for the first message whose Section 4 matches category and paramNum.
//
// Implements WMO GRIB2 Data Representation Template 5.0 (simple packing) directly,
// reading BinaryScale and DecimalScale as int16 as the spec requires.
// nilsmagnus/grib reads them as uint16, causing overflow to +Inf for negative scale factors.
func decodeGrib2Value(data []byte, category, paramNum int) (float64, error) {
	// GRIB2 Section 0 (Indicator Section) — always 16 bytes, WMO spec §3:
	//   bytes 0–3 : "GRIB" magic
	//   bytes 4–5 : reserved (0)
	//   byte  6   : discipline (0 = Meteorological, 1 = Hydrological, …)
	//   byte  7   : edition number (2 for GRIB2)
	//   bytes 8–15: total message length (uint64, big-endian)
	if len(data) < 16 || string(data[0:4]) != "GRIB" || data[7] != 2 {
		return 0, fmt.Errorf("GRIB2: invalid stream header")
	}

	// Section 0 is always 16 bytes; variable-length sections follow.
	pos := 16

	var (
		foundCategory, foundParam uint8
		ref                       float64
		binScale, decScale        int16
		bitsPerValue              uint8
		tmpl5Ready                bool
		bitmapIndicator           uint8 = 255 // 255 = no bitmap present
		bitmap                    []byte
	)

	for pos+4 < len(data) {
		if pos+4 <= len(data) && string(data[pos:pos+4]) == "7777" {
			break
		}
		if pos+5 > len(data) {
			break
		}
		secLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if secLen < 5 || pos+secLen > len(data) {
			break
		}
		secNum := data[pos+4]

		switch secNum {
		case 4: // Product Definition — Template 4.0: cat at octet 10, param at octet 11
			if secLen >= 12 {
				foundCategory = data[pos+9]
				foundParam = data[pos+10]
			}

		case 5: // Data Representation — Template 5.0: simple packing
			if secLen >= 21 && binary.BigEndian.Uint16(data[pos+9:pos+11]) == 0 {
				ref = float64(math.Float32frombits(binary.BigEndian.Uint32(data[pos+11 : pos+15])))
				binScale = int16(binary.BigEndian.Uint16(data[pos+15 : pos+17])) // must be signed
				decScale = int16(binary.BigEndian.Uint16(data[pos+17 : pos+19])) // must be signed
				bitsPerValue = data[pos+19]
				tmpl5Ready = true
			}

		case 6: // Bitmap Section
			bitmapIndicator = data[pos+5]
			if bitmapIndicator == 0 && secLen > 6 {
				bitmap = data[pos+6 : pos+secLen]
			}

		case 7: // Data Section
			if int(foundCategory) != category || int(foundParam) != paramNum {
				break
			}
			if !tmpl5Ready {
				return 0, fmt.Errorf("GRIB2: Section 5 not found before Section 7")
			}
			return grib2DecodePacked(data[pos+5:pos+secLen], ref, binScale, decScale, bitsPerValue, bitmapIndicator, bitmap)
		}

		pos += secLen
	}

	return 0, fmt.Errorf("GRIB2: no message with category=%d param=%d", category, paramNum)
}

// grib2DecodePacked decodes simple-packed GRIB2 data (Template 5.0) and returns
// the mean of valid (non-bitmap-masked) values.
func grib2DecodePacked(packed []byte, ref float64, binScale, decScale int16, bits uint8, bitmapIndicator uint8, bitmap []byte) (float64, error) {
	if bits == 0 {
		// Constant field: all values equal the reference value.
		return ref * math.Pow(10.0, -float64(decScale)), nil
	}

	bscale := math.Pow(2.0, float64(binScale))
	dscale := math.Pow(10.0, -float64(decScale))

	// With a bitmap (indicator=0): nGridPoints = len(bitmap)*8;
	// packed integers correspond only to set bitmap bits.
	// Without bitmap: every integer in 'packed' is a valid value.
	nGrid := (len(packed) * 8) / int(bits)
	if bitmapIndicator == 0 && len(bitmap) > 0 {
		nGrid = len(bitmap) * 8
	}

	sum, count := 0.0, 0
	packedIdx := 0
	for i := 0; i < nGrid; i++ {
		if bitmapIndicator == 0 && len(bitmap) > 0 {
			byteIdx, bitIdx := i/8, 7-(i%8)
			if byteIdx >= len(bitmap) || (bitmap[byteIdx]>>uint(bitIdx))&1 == 0 {
				continue // missing grid point
			}
		}
		x := grib2ReadBits(packed, packedIdx*int(bits), int(bits))
		packedIdx++
		sum += (ref + float64(x)*bscale) * dscale
		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("GRIB2: no valid values (all bitmap-masked)")
	}
	return sum / float64(count), nil
}

// grib2ReadBits reads nBits bits from data starting at bitOffset (MSB-first packing).
func grib2ReadBits(data []byte, bitOffset, nBits int) uint64 {
	var result uint64
	for i := 0; i < nBits; i++ {
		byteIdx := (bitOffset + i) / 8
		bitIdx := 7 - ((bitOffset + i) % 8)
		if byteIdx < len(data) {
			result = (result << 1) | uint64((data[byteIdx]>>uint(bitIdx))&1)
		}
	}
	return result
}

// deriveCondition maps numeric forecast values to a condition string.
func deriveCondition(tempC, precipMm, windKmh float64) string {
	switch {
	case precipMm > 5.0 && tempC <= 2.0:
		return "snow"
	case precipMm > 5.0:
		return "rain"
	case precipMm > 1.0 || windKmh > 40:
		return "partly_cloudy"
	case windKmh > 70:
		return "storm"
	default:
		return "sunny"
	}
}
