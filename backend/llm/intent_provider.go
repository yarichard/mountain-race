package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"google.golang.org/genai"
)

// IntentProvider parses natural-language race descriptions into structured intent.
type IntentProvider interface {
	ParseRaceIntent(ctx context.Context, text, lang string) (*ParseIntentResult, error)
}

// NewIntentProvider returns the IntentProvider selected by the INTENT_LLM_PROVIDER env var.
// Supported values: "ollama", "openai", "gemini" (default).
func NewIntentProvider() IntentProvider {
	switch os.Getenv("INTENT_LLM_PROVIDER") {
	case "ollama":
		return &ollamaIntentProvider{}
	case "openai":
		return &openaiIntentProvider{}
	default:
		return &geminiIntentProvider{}
	}
}

// intentModel returns the model for intent parsing.
// INTENT_LLM_MODEL overrides the provider-specific default.
func intentModel(providerDefault string) string {
	if m := os.Getenv("INTENT_LLM_MODEL"); m != "" {
		return m
	}
	return providerDefault
}

// parseIntentJSON extracts and unmarshals a ParseIntentResult from raw LLM text.
func parseIntentJSON(raw string) (*ParseIntentResult, error) {
	match := jsonObjectRe.FindString(raw)
	if match == "" {
		return nil, fmt.Errorf("no JSON object found in LLM response")
	}
	var result ParseIntentResult
	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return nil, fmt.Errorf("parsing intent JSON: %w", err)
	}
	return &result, nil
}

// --- Gemini ---

type geminiIntentProvider struct{}

func (p *geminiIntentProvider) ParseRaceIntent(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	return ParseRaceIntentGemini(ctx, text, lang)
}

func ParseRaceIntentGemini(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	model := intentModel(geminiModel())
	prompt := intentSystemPrompt(lang) + "\n\n" + intentUserPrompt(text)
	result, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("calling Gemini API: %w", err)
	}

	return parseIntentJSON(result.Text())
}

// --- OpenAI ---

type openaiIntentProvider struct{}

func (p *openaiIntentProvider) ParseRaceIntent(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	return ParseRaceIntentOpenAI(ctx, text, lang)
}

func ParseRaceIntentOpenAI(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	reqBody := openAIChatRequest{
		Model: intentModel(openAIModel()),
		Messages: []openAIChatMessage{
			{Role: "system", Content: intentSystemPrompt(lang)},
			{Role: "user", Content: intentUserPrompt(text)},
		},
		Temperature: 0,
		MaxTokens:   1024,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building OpenAI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI returned status %d: %s", resp.StatusCode, raw)
	}

	raw, _ := io.ReadAll(resp.Body)
	var openAIResp openAIChatResponse
	if err := json.Unmarshal(raw, &openAIResp); err != nil {
		return nil, fmt.Errorf("parsing OpenAI response: %w", err)
	}
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	return parseIntentJSON(openAIResp.Choices[0].Message.Content)
}

// --- Ollama ---

type ollamaIntentProvider struct{}

func (p *ollamaIntentProvider) ParseRaceIntent(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	return ParseRaceIntentOllama(ctx, text, lang)
}

func ParseRaceIntentOllama(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	// Use a hard deadline independent of the HTTP request context.
	ollamaCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	reqBody := ollamaChatRequest{
		Model: intentModel(ollamaModel()),
		Messages: []ollamaChatMessage{
			{Role: "system", Content: intentSystemPrompt(lang)},
			{Role: "user", Content: intentUserPrompt(text)},
		},
		Stream: false,
		Options: map[string]any{
			"num_predict": 1024,
			"temperature": 0,
		},
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ollamaCtx, http.MethodPost, ollamaURL()+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	raw, _ := io.ReadAll(resp.Body)
	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parsing ollama response: %w", err)
	}

	return parseIntentJSON(ollamaResp.Message.Content)
}
