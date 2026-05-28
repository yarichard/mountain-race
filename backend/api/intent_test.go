package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"mountain-race/llm"
)

func TestParseIntent_InvalidJSON(t *testing.T) {
	r := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestParseIntent_EmptyText(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"text": "  ", "lang": "fr"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty text, got %d", w.Code)
	}
}

func TestParseIntent_LLMError(t *testing.T) {
	orig := intentParse
	intentParse = func(_ context.Context, _, _ string) (*llm.ParseIntentResult, error) {
		return nil, fmt.Errorf("LLM unavailable")
	}
	defer func() { intentParse = orig }()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"text": "Une course autour de Chamonix", "lang": "fr"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestParseIntent_Success(t *testing.T) {
	orig := intentParse
	intentParse = func(_ context.Context, text, lang string) (*llm.ParseIntentResult, error) {
		return &llm.ParseIntentResult{
			Intent: llm.RaceIntent{
				Location:     "Chamonix",
				LocationType: "name",
				RaceType:     "ridge_hike",
				Date:         "2025-07-15",
				Difficulty:   "AD",
			},
			Missing: []string{},
		}, nil
	}
	defer func() { intentParse = orig }()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"text": "Une course de crête AD à Chamonix le 15 juillet", "lang": "fr"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp llm.ParseIntentResult
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Intent.Location != "Chamonix" {
		t.Errorf("expected Chamonix, got %q", resp.Intent.Location)
	}
	if resp.Intent.RaceType != "ridge_hike" {
		t.Errorf("expected ridge_hike, got %q", resp.Intent.RaceType)
	}
}

func TestParseIntent_UsesAcceptLanguageFallback(t *testing.T) {
	var capturedLang string
	orig := intentParse
	intentParse = func(_ context.Context, _, lang string) (*llm.ParseIntentResult, error) {
		capturedLang = lang
		return &llm.ParseIntentResult{Intent: llm.RaceIntent{}, Missing: []string{"location", "race_type", "date"}}, nil
	}
	defer func() { intentParse = orig }()

	r := newTestRouter()
	// no lang in body → fallback to Accept-Language header
	body, _ := json.Marshal(map[string]any{"text": "some race"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if capturedLang != "en" {
		t.Errorf("expected lang=en, got %q", capturedLang)
	}
}

func TestParseIntent_WithParticipants(t *testing.T) {
	orig := intentParse
	intentParse = func(_ context.Context, _, _ string) (*llm.ParseIntentResult, error) {
		return &llm.ParseIntentResult{
			Intent: llm.RaceIntent{
				Location:     "Chamonix",
				LocationType: "name",
				RaceType:     "multipitch",
				Date:         "2025-07-15",
				Participants: []llm.IntentParticipant{
					{Name: "Alice", ClimbingLevel: "6a"},
					{Name: "Bob", ClimbingLevel: "5c"},
				},
			},
			Missing: []string{},
		}, nil
	}
	defer func() { intentParse = orig }()

	r := newTestRouter()
	body, _ := json.Marshal(map[string]any{"text": "Une course avec Alice 6a et Bob 5c", "lang": "fr"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/intent/parse", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp llm.ParseIntentResult
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Intent.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(resp.Intent.Participants))
	}
}
