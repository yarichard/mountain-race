package llm

import (
	"context"
	"os"
)

// Provider is the common interface for all LLM backends.
type Provider interface {
	ExtractEquipment(ctx context.Context, gearText, lang string) ([]EquipmentItem, error)
}

// NewProvider returns the Provider selected by the LLM_PROVIDER env var.
// Supported values: "ollama", "openai", "gemini" (default).
func NewProvider() Provider {
	switch os.Getenv("LLM_PROVIDER") {
	case "ollama":
		return &ollamaProvider{}
	case "openai":
		return &openaiProvider{}
	default:
		return &geminiProvider{}
	}
}
