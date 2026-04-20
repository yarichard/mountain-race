package api

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"mountain-race/meteo"
)

// GetWeather handles GET /api/weather?lat=&lon=&date=
func GetWeather(c *gin.Context) {
	latStr := c.Query("lat")
	lonStr := c.Query("lon")
	dateStr := c.Query("date")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lat"})
		return
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lon"})
		return
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date, expected YYYY-MM-DD"})
		return
	}

	forecast, hourly, err := meteo.FetchWeather(lat, lon, date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	avalanche, _ := meteo.AvalancheForecast(lat, lon, date)

	c.JSON(http.StatusOK, gin.H{
		"forecast":  forecast,
		"avalanche": avalanche,
		"hourly":    hourly,
	})
}

var allowedImageTypes = map[string]bool{
	"montagne-risques":  true,
	"apercu-meteo":      true,
	"sept-derniers-jours": true,
}

// GetAvalancheImage proxies a DPBRA massif image (requires Bearer auth).
// GET /api/avalanche/image?massif_id=X&type=montagne-risques
func GetAvalancheImage(c *gin.Context) {
	massifIDStr := c.Query("massif_id")
	imageType := c.Query("type")

	massifID, err := strconv.Atoi(massifIDStr)
	if err != nil || massifID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid massif_id"})
		return
	}
	if !allowedImageTypes[imageType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type"})
		return
	}

	var buf bytes.Buffer
	ct, err := meteo.ProxyMassifImage(&buf, massifID, imageType)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if ct == "" {
		ct = "image/png"
	}
	c.Data(http.StatusOK, ct, buf.Bytes())
}
