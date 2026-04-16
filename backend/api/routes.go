package api

import (
	"net/http"

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
}

// SearchRoutes handles POST /api/routes/search
func SearchRoutes(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := camptocamp.Search(camptocamp.SearchRequest{
		Location:     req.Location,
		LocationType: req.LocationType,
		RaceType:     req.RaceType,
		Difficulty:   req.Difficulty,
		Date:         req.Date,
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
	detail, err := camptocamp.GetDetail(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, detail)
}
