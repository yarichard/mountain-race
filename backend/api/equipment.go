package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mountain-race/camptocamp"
	"mountain-race/llm"
	"os"
)

type extractEquipmentRequest struct {
	GearText string `json:"gear_text"`
}

// equipExtract can be replaced in tests to avoid a real LLM call.
// The provider is selected at runtime via the LLM_PROVIDER env var ("gemini" or "ollama", default "gemini").
var equipExtract = func(ctx context.Context, gearText, lang string) ([]llm.EquipmentItem, error) {
	if os.Getenv("LLM_PROVIDER") == "ollama" {
		return llm.ExtractEquipmentOllama(ctx, gearText, lang)
	}
	return llm.ExtractEquipmentGemini(ctx, gearText, lang)
}

// ExtractEquipment handles POST /api/equipment/extract
func ExtractEquipment(c *gin.Context) {
	var req extractEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.GearText) == "" {
		c.JSON(http.StatusOK, gin.H{"equipment": []camptocamp.Equipment{}})
		return
	}

	lang := preferredLang(c.GetHeader("Accept-Language"))
	items, err := equipExtract(c.Request.Context(), req.GearText, lang)
	if err != nil || len(items) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	equipment := make([]camptocamp.Equipment, len(items))
	for i, item := range items {
		equipment[i] = camptocamp.Equipment{Item: item.Name, Quantity: item.Quantity, Notes: item.Notes}
	}
	c.JSON(http.StatusOK, gin.H{"equipment": equipment})
}
