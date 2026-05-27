package llm

import (
	"context"
	"testing"
)

func TestGeminiModel_Default(t *testing.T) {
	t.Setenv("GEMINI_MODEL", "")
	got := geminiModel()
	if got != "gemini-2.5-flash-lite" {
		t.Errorf("expected default gemini model, got %q", got)
	}
}

func TestGeminiModel_Override(t *testing.T) {
	t.Setenv("GEMINI_MODEL", "gemini-1.5-pro")
	got := geminiModel()
	if got != "gemini-1.5-pro" {
		t.Errorf("expected gemini-1.5-pro, got %q", got)
	}
}

func TestExtractEquipmentGemini_EmptyGearText(t *testing.T) {
	// Empty gear text returns nil immediately without calling the API
	items, err := ExtractEquipmentGemini(context.Background(), "", "fr")
	if err != nil {
		t.Fatalf("unexpected error for empty gear text: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil for empty gear text, got %v", items)
	}
}

func TestExtractEquipmentGemini_NoAPIKey_ReturnsError(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	// With no API key, the Gemini client creation should fail
	_, err := ExtractEquipmentGemini(context.Background(), "rope, harness, helmet", "fr")
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is not set")
	}
}

func TestGeminiProvider_DispatchesToExtractEquipmentGemini(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	p := &geminiProvider{}
	// Should propagate the error from ExtractEquipmentGemini (no API key)
	_, err := p.ExtractEquipment(context.Background(), "rope", "fr")
	if err == nil {
		t.Fatal("expected error from geminiProvider when API key is missing")
	}
}
