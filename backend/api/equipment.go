package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mountain-race/camptocamp"
	"mountain-race/llm"
)

type extractEquipmentRequest struct {
	GearText string `json:"gear_text"`
}

// ExtractEquipment handles POST /api/equipment/extract
func ExtractEquipment(c *gin.Context) {
	var req extractEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lang := preferredLang(c.GetHeader("Accept-Language"))
	items, err := llm.ExtractEquipment(c.Request.Context(), req.GearText, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	equipment := make([]camptocamp.Equipment, len(items))
	for i, item := range items {
		equipment[i] = camptocamp.Equipment{Item: item.Name, Quantity: item.Quantity, Notes: item.Notes}
	}
	c.JSON(http.StatusOK, gin.H{"equipment": equipment})
}
