package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mountain-race/llm"
)

type parseIntentRequest struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

// intentParse can be replaced in tests to avoid a real LLM call.
var intentParse = func(ctx context.Context, text, lang string) (*llm.ParseIntentResult, error) {
	return llm.NewProvider().ParseRaceIntent(ctx, text, lang)
}

// ParseIntentHandler handles POST /api/intent/parse
func ParseIntentHandler(c *gin.Context) {
	var req parseIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	lang := req.Lang
	if lang == "" {
		lang = preferredLang(c.GetHeader("Accept-Language"))
	}

	result, err := intentParse(c.Request.Context(), req.Text, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
