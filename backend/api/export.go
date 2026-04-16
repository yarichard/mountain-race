package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"mountain-race/pdf"
)

// ExportPDF handles POST /api/export/pdf
func ExportPDF(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	pdfBytes, err := pdf.Generate(c.Writer, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=mountain-race.pdf")
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}
