package api

import "github.com/gin-gonic/gin"

// Register wires all API routes onto the router.
func Register(r *gin.Engine) {
	g := r.Group("/api")

	g.POST("/routes/search", SearchRoutes)
	g.GET("/routes/:id", GetRoute)
	g.GET("/weather", GetWeather)
	g.POST("/export/pdf", ExportPDF)
}
