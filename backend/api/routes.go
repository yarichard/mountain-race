package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mountain-race/camptocamp"
)

type searchRequest struct {
	Location     string                  `json:"location"`
	LocationType string                  `json:"location_type"`
	RaceType     string                  `json:"race_type"`
	Difficulty   string                  `json:"difficulty"`
	Date         string                  `json:"date"`
	Participants []camptocamp.Participant `json:"participants"`
	AllowAbove   bool                    `json:"allow_above"`
	RadiusKm     int                     `json:"radius_km"`
}

// preferredLang extracts the primary language code from an Accept-Language header value.
// e.g. "fr-FR,fr;q=0.9,en;q=0.8" → "fr"
func preferredLang(acceptLang string) string {
	if acceptLang == "" {
		return "fr"
	}
	tag := strings.TrimSpace(strings.Split(strings.Split(acceptLang, ",")[0], ";")[0])
	if idx := strings.Index(tag, "-"); idx > 0 {
		tag = tag[:idx]
	}
	return strings.ToLower(tag)
}

// SearchRoutes handles POST /api/routes/search
func SearchRoutes(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lang := preferredLang(c.GetHeader("Accept-Language"))
	results, err := camptocamp.Search(camptocamp.SearchRequest{
		Location:     req.Location,
		LocationType: req.LocationType,
		RaceType:     req.RaceType,
		Difficulty:   req.Difficulty,
		Date:         req.Date,
		AllowAbove:   req.AllowAbove,
		Lang:         lang,
		Participants: req.Participants,
		RadiusKm:     req.RadiusKm,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"routes": results})
}

// GetRoute handles GET /api/routes/:id
func GetRoute(c *gin.Context) {
	id := c.Param("id")
	lang := preferredLang(c.GetHeader("Accept-Language"))
	detail, err := camptocamp.GetDetail(c.Request.Context(), id, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}
