package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

var openAIBaseURL = "https://api.openai.com/v1/chat/completions" // overridable in tests

type openaiProvider struct{}

func (p *openaiProvider) ExtractEquipment(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	return ExtractEquipmentOpenAI(ctx, gearText, lang)
}

func (p *openaiProvider) ParseRaceIntent(ctx context.Context, text, lang string) (*ParseIntentResult, error) {
	return ParseRaceIntentOpenAI(ctx, text, lang)
}

func (p *openaiProvider) ParseDuration(ctx context.Context, description, lang string) (*DurationResult, error) {
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	reqBody := openAIChatRequest{
		Model: openAIModel(),
		Messages: []openAIChatMessage{
			{Role: "system", Content: durationSystemPrompt(lang)},
			{Role: "user", Content: durationUserPrompt(description, lang)},
		},
		Temperature: 0,
		MaxTokens:   512,
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
	return parseDurationJSON(openAIResp.Choices[0].Message.Content)
}

func openAIModel() string {
	if m := os.Getenv("OPENAI_MODEL"); m != "" {
		return m
	}
	return "gpt-4o-mini"
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
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
		Model: openAIModel(),
		Messages: []openAIChatMessage{
			{Role: "system", Content: intentSystemPrompt(lang)},
			{Role: "user", Content: intentUserPrompt(text, lang)},
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

func ExtractEquipmentOpenAI(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	if gearText == "" {
		return nil, nil
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("OPENAI_API_KEY not set, cannot call OpenAI API")
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	reqBody := openAIChatRequest{
		Model: openAIModel(),
		Messages: []openAIChatMessage{
			{Role: "system", Content: equipmentSystemPrompt()},
			{Role: "user", Content: equipmentUserPrompt(gearText)},
		},
		Temperature: 0,
		MaxTokens:   512,
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

	match := jsonArrayRe.FindString(openAIResp.Choices[0].Message.Content)
	if match == "" {
		return nil, fmt.Errorf("no JSON array found in OpenAI response")
	}

	var items []EquipmentItem
	if err := json.Unmarshal([]byte(match), &items); err != nil {
		return nil, fmt.Errorf("parsing equipment JSON: %w", err)
	}

	return items, nil
}
