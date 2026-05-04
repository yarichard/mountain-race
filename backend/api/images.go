package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetImage proxies an image from the upstream source.
// GET /api/images?source=CampToCamp&name=<filename>
func GetImage(c *gin.Context) {
	source := c.Query("source")
	name := c.Query("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var upstreamURL string
	switch source {
	case "CampToCamp":
		upstreamURL = "https://media.camptocamp.org/c2corg-active/" + name
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported source"})
		return
	}

	resp, err := http.Get(upstreamURL) //nolint:noctx
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	io.Copy(c.Writer, resp.Body) //nolint:errcheck
}
