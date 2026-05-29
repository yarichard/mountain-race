package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- parseIntentJSON ---

func TestParseIntentJSON_ValidObject(t *testing.T) {
	raw := `{"intent":{"location":"Chamonix","location_type":"name","race_type":"ridge_hike","date":"2025-07-15"},"missing":[]}`
	result, err := parseIntentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.Location != "Chamonix" {
		t.Errorf("expected Chamonix, got %q", result.Intent.Location)
	}
	if result.Intent.RaceType != "ridge_hike" {
		t.Errorf("expected ridge_hike, got %q", result.Intent.RaceType)
	}
	if len(result.Missing) != 0 {
		t.Errorf("expected no missing fields, got %v", result.Missing)
	}
}

func TestParseIntentJSON_WithMissingFields(t *testing.T) {
	raw := `{"intent":{"location":"Chamonix","location_type":"name"},"missing":["race_type","date"]}`
	result, err := parseIntentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Missing) != 2 {
		t.Fatalf("expected 2 missing fields, got %d", len(result.Missing))
	}
	if result.Missing[0] != "race_type" {
		t.Errorf("expected race_type, got %q", result.Missing[0])
	}
}

func TestParseIntentJSON_WithParticipants(t *testing.T) {
	raw := `{"intent":{"location":"Chamonix","location_type":"name","race_type":"multipitch","date":"2025-07-15","participants":[{"name":"Alice","climbing_level":"6a"}]},"missing":[]}`
	result, err := parseIntentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Intent.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(result.Intent.Participants))
	}
	if result.Intent.Participants[0].Name != "Alice" {
		t.Errorf("expected Alice, got %q", result.Intent.Participants[0].Name)
	}
}

func TestParseIntentJSON_NoObjectInResponse(t *testing.T) {
	_, err := parseIntentJSON("Sorry, I cannot help.")
	if err == nil {
		t.Fatal("expected error when no JSON object in response")
	}
}

func TestParseIntentJSON_InvalidJSON(t *testing.T) {
	_, err := parseIntentJSON("{broken json")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

// --- ParseRaceIntentOpenAI ---

func mockOpenAIIntentServer(t *testing.T, responseContent string, statusCode int) *httptest.Server {
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

func TestParseRaceIntentOpenAI_ParsesValidJSON(t *testing.T) {
	content := `{"intent":{"location":"Chamonix","location_type":"name","race_type":"ridge_hike","date":"2025-07-15","difficulty":"AD"},"missing":[]}`
	srv := mockOpenAIIntentServer(t, content, http.StatusOK)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	result, err := ParseRaceIntentOpenAI(context.Background(), "Une course de crête AD autour de Chamonix le 15 juillet", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.Location != "Chamonix" {
		t.Errorf("expected Chamonix, got %q", result.Intent.Location)
	}
	if result.Intent.Difficulty != "AD" {
		t.Errorf("expected AD, got %q", result.Intent.Difficulty)
	}
	if len(result.Missing) != 0 {
		t.Errorf("expected no missing fields, got %v", result.Missing)
	}
}

func TestParseRaceIntentOpenAI_EmptyText(t *testing.T) {
	_, err := ParseRaceIntentOpenAI(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestParseRaceIntentOpenAI_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := ParseRaceIntentOpenAI(context.Background(), "some race description", "fr")
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestParseRaceIntentOpenAI_HTTPError(t *testing.T) {
	srv := mockOpenAIIntentServer(t, "", http.StatusTooManyRequests)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := ParseRaceIntentOpenAI(context.Background(), "some race", "fr")
	if err == nil {
		t.Fatal("expected error on HTTP error")
	}
}

func TestParseRaceIntentOpenAI_NoJSONInResponse(t *testing.T) {
	srv := mockOpenAIIntentServer(t, "I cannot help with that.", http.StatusOK)
	defer srv.Close()

	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := ParseRaceIntentOpenAI(context.Background(), "some race", "fr")
	if err == nil {
		t.Fatal("expected error when no JSON in response")
	}
}

// --- ParseRaceIntentOllama ---

func TestParseRaceIntentOllama_ParsesValidJSON(t *testing.T) {
	content := `{"intent":{"location":"Chamonix","location_type":"name","race_type":"hike","date":"2025-08-01"},"missing":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
		})
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	result, err := ParseRaceIntentOllama(context.Background(), "Une randonnée à Chamonix le 1er août", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.RaceType != "hike" {
		t.Errorf("expected hike, got %q", result.Intent.RaceType)
	}
}

func TestParseRaceIntentOllama_EmptyText(t *testing.T) {
	_, err := ParseRaceIntentOllama(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestParseRaceIntentOllama_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	_, err := ParseRaceIntentOllama(context.Background(), "some race", "fr")
	if err == nil {
		t.Fatal("expected error on HTTP error")
	}
}

// --- ambiguous race_type ---

func TestParseIntentJSON_AmbiguousRaceType(t *testing.T) {
	raw := `{"intent":{"location":"Chamonix","location_type":"name","date":"2025-07-15"},"missing":["race_type"]}`
	result, err := parseIntentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.RaceType != "" {
		t.Errorf("expected empty race_type, got %q", result.Intent.RaceType)
	}
	if len(result.Missing) != 1 || result.Missing[0] != "race_type" {
		t.Errorf("expected [race_type] in missing, got %v", result.Missing)
	}
}
