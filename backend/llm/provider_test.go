package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewProvider_DefaultIsGemini(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "")
	p := NewProvider()
	if _, ok := p.(*geminiProvider); !ok {
		t.Errorf("expected geminiProvider, got %T", p)
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "ollama")
	p := NewProvider()
	if _, ok := p.(*ollamaProvider); !ok {
		t.Errorf("expected ollamaProvider, got %T", p)
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "openai")
	p := NewProvider()
	if _, ok := p.(*openaiProvider); !ok {
		t.Errorf("expected openaiProvider, got %T", p)
	}
}

func TestNewProvider_UnknownFallsBackToGemini(t *testing.T) {
	t.Setenv("LLM_PROVIDER", "nonexistent")
	p := NewProvider()
	if _, ok := p.(*geminiProvider); !ok {
		t.Errorf("expected geminiProvider as fallback, got %T", p)
	}
}

// --- OpenAI provider ---

func mockOpenAIServer(t *testing.T, responseContent string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{
					map[string]any{
						"message": map[string]any{
							"role":    "assistant",
							"content": responseContent,
						},
					},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"error": "bad request"})
		}
	}))
}

func TestExtractEquipmentOpenAI_ParsesValidJSON(t *testing.T) {
	content := `[{"name":"Corde 60m","quantity":1,"notes":"obligatoire"},{"name":"Casque","quantity":1,"notes":"obligatoire"}]`
	srv := mockOpenAIServer(t, content, http.StatusOK)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	items, err := ExtractEquipmentOpenAI(context.Background(), "Corde 60m, casque", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "Corde 60m" {
		t.Errorf("item[0].Name: got %q, want Corde 60m", items[0].Name)
	}
}

func TestExtractEquipmentOpenAI_EmptyGearText(t *testing.T) {
	items, err := ExtractEquipmentOpenAI(context.Background(), "", "fr")
	if err != nil {
		t.Fatalf("unexpected error for empty gear text: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil for empty gear text, got %v", items)
	}
}

func TestExtractEquipmentOpenAI_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	_, err := ExtractEquipmentOpenAI(context.Background(), "rope, harness", "fr")
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestExtractEquipmentOpenAI_HTTPError(t *testing.T) {
	srv := mockOpenAIServer(t, "", http.StatusTooManyRequests)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := ExtractEquipmentOpenAI(context.Background(), "rope", "fr")
	if err == nil {
		t.Fatal("expected error on HTTP error response")
	}
}

func TestExtractEquipmentOpenAI_NoJSONInResponse(t *testing.T) {
	srv := mockOpenAIServer(t, "Sorry, I cannot help with that.", http.StatusOK)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := ExtractEquipmentOpenAI(context.Background(), "rope", "fr")
	if err == nil {
		t.Fatal("expected error when no JSON array in response")
	}
}

func TestExtractEquipmentOpenAI_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := ExtractEquipmentOpenAI(context.Background(), "rope", "fr")
	if err == nil {
		t.Fatal("expected error when choices array is empty")
	}
}

func TestOllamaProvider_DispatchesToExtractEquipmentOllama(t *testing.T) {
	srv := mockOllamaServer(t, `[{"name":"Corde","quantity":1,"notes":"obligatoire"}]`)
	defer srv.Close()
	// OLLAMA_URL is set by mockOllamaServer via t.Setenv

	p := &ollamaProvider{}
	items, err := p.ExtractEquipment(context.Background(), "corde", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items, got none")
	}
}

func TestOpenAIProvider_DispatchesToExtractEquipmentOpenAI(t *testing.T) {
	content := `[{"name":"Casque","quantity":1,"notes":"obligatoire"}]`
	srv := mockOpenAIServer(t, content, http.StatusOK)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	items, err := p.ExtractEquipment(context.Background(), "casque", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items, got none")
	}
}

func TestOllama_OllamaStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	_, err := ExtractEquipmentOllama(context.Background(), "rope", "fr")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}
