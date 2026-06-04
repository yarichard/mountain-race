package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- parseDurationJSON ---

func TestParseDurationJSON_ValidObject(t *testing.T) {
	raw := `{"total_hours":7.5,"confidence":"high","steps":[{"label":"Approach","hours":1.5},{"label":"Climb","hours":4},{"label":"Descent","hours":2}]}`
	result, err := parseDurationJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 7.5 {
		t.Errorf("expected TotalHours=7.5, got %v", result.TotalHours)
	}
	if result.Confidence != "high" {
		t.Errorf("expected Confidence=high, got %q", result.Confidence)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Label != "Approach" {
		t.Errorf("expected step[0].Label=Approach, got %q", result.Steps[0].Label)
	}
}

func TestParseDurationJSON_ZeroHours(t *testing.T) {
	raw := `{"total_hours":0,"confidence":"low","steps":[]}`
	result, err := parseDurationJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 0 {
		t.Errorf("expected TotalHours=0, got %v", result.TotalHours)
	}
}

func TestParseDurationJSON_NoObjectInResponse(t *testing.T) {
	_, err := parseDurationJSON("I cannot determine the duration.")
	if err == nil {
		t.Fatal("expected error when no JSON object found")
	}
}

func TestParseDurationJSON_InvalidJSON(t *testing.T) {
	_, err := parseDurationJSON("{broken json")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestParseDurationJSON_EmbeddedInText(t *testing.T) {
	raw := `Here is my estimate: {"total_hours":5,"confidence":"medium","steps":[]} some trailing text`
	result, err := parseDurationJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 5 {
		t.Errorf("expected TotalHours=5, got %v", result.TotalHours)
	}
}

// --- durationSystemPrompt and durationUserPrompt ---

func TestDurationSystemPrompt_French(t *testing.T) {
	prompt := durationSystemPrompt("fr")
	if prompt == "" {
		t.Fatal("expected non-empty prompt for 'fr'")
	}
	// Should be in French
	if len(prompt) < 100 {
		t.Error("French prompt seems too short")
	}
}

func TestDurationSystemPrompt_English(t *testing.T) {
	prompt := durationSystemPrompt("en")
	if prompt == "" {
		t.Fatal("expected non-empty prompt for 'en'")
	}
	// English and French prompts should differ
	if prompt == durationSystemPrompt("fr") {
		t.Error("English and French prompts should be different")
	}
}

func TestDurationUserPrompt_French(t *testing.T) {
	p := durationUserPrompt("Belle voie classique de 6 heures.", "fr")
	if p == "" {
		t.Fatal("expected non-empty user prompt")
	}
}

func TestDurationUserPrompt_English(t *testing.T) {
	p := durationUserPrompt("Classic route taking about 6 hours.", "en")
	if p == "" {
		t.Fatal("expected non-empty user prompt for 'en'")
	}
	pFr := durationUserPrompt("Same description", "fr")
	if p == pFr {
		t.Error("English and French user prompts should differ")
	}
}

// --- intentUserPrompt ---

func TestIntentUserPrompt_French(t *testing.T) {
	p := intentUserPrompt("grande voie à Chamonix le 30/06", "fr")
	if p == "" {
		t.Fatal("expected non-empty intent user prompt")
	}
}

func TestIntentUserPrompt_English(t *testing.T) {
	p := intentUserPrompt("big wall near Chamonix on 30/06", "en")
	if p == "" {
		t.Fatal("expected non-empty intent user prompt for 'en'")
	}
	pFr := intentUserPrompt("big wall near Chamonix on 30/06", "fr")
	if p == pFr {
		t.Error("English and French intent user prompts should differ")
	}
}

// --- ParseDuration via Ollama ---

func ollamaDurationServer(t *testing.T, content string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			json.NewEncoder(w).Encode(map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
				"done": true,
			})
		}
	}))
}

func TestOllamaProvider_ParseDuration_ValidResponse(t *testing.T) {
	content := `{"total_hours":5.5,"confidence":"medium","steps":[{"label":"Approche","hours":1.5},{"label":"Escalade","hours":4}]}`
	srv := ollamaDurationServer(t, content, http.StatusOK)
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	p := &ollamaProvider{}
	result, err := p.ParseDuration(context.Background(), "Belle voie de 5h30 avec 1h30 d'approche.", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 5.5 {
		t.Errorf("expected TotalHours=5.5, got %v", result.TotalHours)
	}
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
}

func TestOllamaProvider_ParseDuration_EmptyDescription(t *testing.T) {
	p := &ollamaProvider{}
	_, err := p.ParseDuration(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestOllamaProvider_ParseDuration_HTTPError(t *testing.T) {
	srv := ollamaDurationServer(t, "", http.StatusServiceUnavailable)
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	p := &ollamaProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route description.", "fr")
	if err == nil {
		t.Fatal("expected error on HTTP error response")
	}
}

func TestOllamaProvider_ParseDuration_NoJSONInResponse(t *testing.T) {
	srv := ollamaDurationServer(t, "I cannot estimate the duration.", http.StatusOK)
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	p := &ollamaProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route description.", "fr")
	if err == nil {
		t.Fatal("expected error when no JSON in response")
	}
}

func TestOllamaProvider_ParseDuration_EnglishPrompt(t *testing.T) {
	content := `{"total_hours":6,"confidence":"high","steps":[]}`
	srv := ollamaDurationServer(t, content, http.StatusOK)
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	p := &ollamaProvider{}
	result, err := p.ParseDuration(context.Background(), "Classic 6-hour route.", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 6 {
		t.Errorf("expected TotalHours=6, got %v", result.TotalHours)
	}
}

// --- ParseDuration via OpenAI ---

func openAIDurationServer(t *testing.T, content string, statusCode int) *httptest.Server {
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
							"content": content,
						},
					},
				},
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"error": "error"})
		}
	}))
}

func TestOpenAIProvider_ParseDuration_ValidResponse(t *testing.T) {
	content := `{"total_hours":8,"confidence":"high","steps":[{"label":"Approach","hours":2},{"label":"Climb","hours":5},{"label":"Descent","hours":1}]}`
	srv := openAIDurationServer(t, content, http.StatusOK)
	defer srv.Close()
	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	result, err := p.ParseDuration(context.Background(), "Classic 8-hour route.", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHours != 8 {
		t.Errorf("expected TotalHours=8, got %v", result.TotalHours)
	}
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(result.Steps))
	}
}

func TestOpenAIProvider_ParseDuration_EmptyDescription(t *testing.T) {
	p := &openaiProvider{}
	_, err := p.ParseDuration(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestOpenAIProvider_ParseDuration_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	p := &openaiProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route description.", "fr")
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestOpenAIProvider_ParseDuration_HTTPError(t *testing.T) {
	srv := openAIDurationServer(t, "", http.StatusTooManyRequests)
	defer srv.Close()
	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route.", "fr")
	if err == nil {
		t.Fatal("expected error on HTTP error response")
	}
}

func TestOpenAIProvider_ParseDuration_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()
	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route.", "fr")
	if err == nil {
		t.Fatal("expected error when no choices returned")
	}
}

func TestOpenAIProvider_ParseDuration_NoJSONInResponse(t *testing.T) {
	srv := openAIDurationServer(t, "I cannot estimate the duration.", http.StatusOK)
	defer srv.Close()
	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	_, err := p.ParseDuration(context.Background(), "Some route.", "fr")
	if err == nil {
		t.Fatal("expected error when no JSON in response")
	}
}

// --- Provider interface method dispatch (ParseRaceIntent) ---

func TestOllamaProvider_ParseRaceIntent_Dispatch(t *testing.T) {
	content := `{"intent":{"location":"Chamonix","location_type":"location","race_type":"hike","date":"2025-08-01"},"missing":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{"role": "assistant", "content": content},
		})
	}))
	defer srv.Close()
	t.Setenv("OLLAMA_URL", srv.URL)

	p := &ollamaProvider{}
	result, err := p.ParseRaceIntent(context.Background(), "Randonnée à Chamonix le 1er août", "fr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.RaceType != "hike" {
		t.Errorf("expected hike, got %q", result.Intent.RaceType)
	}
}

func TestOpenAIProvider_ParseRaceIntent_Dispatch(t *testing.T) {
	content := `{"intent":{"location":"Chamonix","location_type":"name","race_type":"ridge_hike","date":"2025-07-15"},"missing":[]}`
	srv := openAIDurationServer(t, content, http.StatusOK)
	defer srv.Close()
	openAIBaseURL = srv.URL + "/v1/chat/completions"
	defer func() { openAIBaseURL = "https://api.openai.com/v1/chat/completions" }()
	t.Setenv("OPENAI_API_KEY", "test-key")

	p := &openaiProvider{}
	result, err := p.ParseRaceIntent(context.Background(), "Ridge hike in Chamonix July 15", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Intent.RaceType != "ridge_hike" {
		t.Errorf("expected ridge_hike, got %q", result.Intent.RaceType)
	}
}

// --- GeminiProvider ParseDuration/ParseRaceIntent with no API key ---

func TestGeminiProvider_ParseDuration_EmptyDescription(t *testing.T) {
	p := &geminiProvider{}
	_, err := p.ParseDuration(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestGeminiProvider_ParseDuration_NoAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	p := &geminiProvider{}
	_, err := p.ParseDuration(context.Background(), "Belle voie classique.", "fr")
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is not set")
	}
}

func TestGeminiProvider_ParseRaceIntent_EmptyText(t *testing.T) {
	p := &geminiProvider{}
	_, err := p.ParseRaceIntent(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestGeminiProvider_ParseRaceIntent_NoAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	p := &geminiProvider{}
	_, err := p.ParseRaceIntent(context.Background(), "Grande voie à Chamonix", "fr")
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is not set")
	}
}

// --- ParseRaceIntentGemini ---

func TestParseRaceIntentGemini_EmptyText(t *testing.T) {
	_, err := ParseRaceIntentGemini(context.Background(), "", "fr")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestParseRaceIntentGemini_NoAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	_, err := ParseRaceIntentGemini(context.Background(), "Une voie classique", "fr")
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is not set")
	}
}

// --- ollamaURL and ollamaModel ---

func TestOllamaURL_Default(t *testing.T) {
	t.Setenv("OLLAMA_URL", "")
	got := ollamaURL()
	if got != "http://host.docker.internal:11434" {
		t.Errorf("expected default ollama URL, got %q", got)
	}
}

func TestOllamaURL_Override(t *testing.T) {
	t.Setenv("OLLAMA_URL", "http://localhost:11434")
	got := ollamaURL()
	if got != "http://localhost:11434" {
		t.Errorf("expected http://localhost:11434, got %q", got)
	}
}

func TestOllamaModel_Default(t *testing.T) {
	t.Setenv("OLLAMA_MODEL", "")
	got := ollamaModel()
	if got != "llama3.2" {
		t.Errorf("expected default ollama model, got %q", got)
	}
}

func TestOllamaModel_Override(t *testing.T) {
	t.Setenv("OLLAMA_MODEL", "mistral")
	got := ollamaModel()
	if got != "mistral" {
		t.Errorf("expected mistral, got %q", got)
	}
}

// --- openAIModel ---

func TestOpenAIModel_Default(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "")
	got := openAIModel()
	if got != "gpt-4o-mini" {
		t.Errorf("expected default openai model, got %q", got)
	}
}

func TestOpenAIModel_Override(t *testing.T) {
	t.Setenv("OPENAI_MODEL", "gpt-4o")
	got := openAIModel()
	if got != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %q", got)
	}
}

// --- parseIntentJSON optional fields filtering ---

func TestParseIntentJSON_FiltersOptionalFromMissing(t *testing.T) {
	// Small models sometimes add optional fields like "difficulty" or "participants" to missing
	raw := `{"intent":{"location":"Chamonix","location_type":"location","race_type":"multipitch"},"missing":["date","difficulty","participants"]}`
	result, err := parseIntentJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only "date" is required; "difficulty" and "participants" should be filtered out
	if len(result.Missing) != 1 || result.Missing[0] != "date" {
		t.Errorf("expected only [date] in missing after filtering, got %v", result.Missing)
	}
}
