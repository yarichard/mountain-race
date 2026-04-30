package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func mockOllamaServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"done": true,
		})
	}))
	t.Setenv("OLLAMA_URL", srv.URL)
	return srv
}

func TestExtractEquipment_ParsesValidJSON(t *testing.T) {
	srv := mockOllamaServer(t, `Here is the list:
[{"name":"Corde 60m","quantity":1,"notes":"mandatory"},{"name":"Dégaines","quantity":12,"notes":"mandatory"},{"name":"Casque","quantity":1,"notes":"mandatory"}]`)
	defer srv.Close()

	items, err := ExtractEquipmentOllama(context.Background(), "Corde 60m, 12 dégaines, casque obligatoire", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Name != "Corde 60m" {
		t.Errorf("item[0].Name: got %q, want %q", items[0].Name, "Corde 60m")
	}
	if items[1].Quantity != 12 {
		t.Errorf("item[1].Quantity: got %d, want 12", items[1].Quantity)
	}
	if items[2].Notes != "mandatory" {
		t.Errorf("item[2].Notes: got %q, want %q", items[2].Notes, "mandatory")
	}
}

func TestExtractEquipment_EmptyGearText(t *testing.T) {
	items, err := ExtractEquipmentOllama(context.Background(), "", "fr")
	if err != nil {
		t.Fatalf("unexpected error for empty gear text: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil for empty gear text, got %v", items)
	}
}

func TestExtractEquipment_OllamaUnreachable(t *testing.T) {
	os.Setenv("OLLAMA_URL", "http://127.0.0.1:1") // nothing listening here
	defer os.Unsetenv("OLLAMA_URL")

	_, err := ExtractEquipmentOllama(context.Background(), "some gear", "fr")
	if err == nil {
		t.Fatal("expected error when ollama is unreachable, got nil")
	}
}

func TestExtractEquipment_InvalidJSONResponse(t *testing.T) {
	srv := mockOllamaServer(t, "Sorry, I cannot parse this.")
	defer srv.Close()

	_, err := ExtractEquipmentOllama(context.Background(), "some gear", "fr")
	if err == nil {
		t.Fatal("expected error when LLM returns no JSON array, got nil")
	}
}

func TestExtractEquipment_EnglishLang(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": `[{"name":"Rope 60m","quantity":1,"notes":"mandatory"}]`,
			},
			"done": true,
		})
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	items, err := ExtractEquipmentOllama(context.Background(), "60m rope", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items, got none")
	}
	messages, _ := capturedBody["messages"].([]any)
	if len(messages) == 0 {
		t.Error("messages were not sent to ollama")
	}
}
