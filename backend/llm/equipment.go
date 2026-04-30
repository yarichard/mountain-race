package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"

	"google.golang.org/genai"
)

// EquipmentItem is the structured output from LLM gear parsing.
type EquipmentItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}

func ollamaURL() string {
	if u := os.Getenv("OLLAMA_URL"); u != "" {
		return u
	}
	return "http://host.docker.internal:11434"
}

func ollamaModel() string {
	if m := os.Getenv("OLLAMA_MODEL"); m != "" {
		return m
	}
	return "llama3.2"
}

func geminiModel() string {
	if m := os.Getenv("GEMINI_MODEL"); m != "" {
		return m
	}
	return "gemini-2.5-flash-lite"
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

func equipmentSystemPrompt() string {
	return `You are a mountain climbing equipment assistant. Parse the following gear description and return a JSON array. Each element must have exactly three fields:
	- "name": equipment name (string, in french)
	- "quantity": number needed (integer, 1 if unspecified)
	- "notes": "optional" or "mandatory" (translated in french), plus any relevant detail (string, in french)
	The name of these equipments are related with the mountain activities. You should only point out personal equipment, for instance quickdraws or rope.
	You should include only equipment you're absolutely sure about. Output ONLY the JSON array, no explanation.`
}

func equipmentUserPrompt(gearText string) string {
	return "Gear description:\n " + gearText
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type OllamaRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream"`
    Options  map[string]any `json:"options"`
}

type OllamaResponse struct {
    Message Message `json:"message"`
}

// ExtractEquipment parses a free-form gear description into a structured list.
// lang is "fr" or "en" and drives the prompt language.
// Returns an error if Ollama is unreachable or returns unparseable output.
func ExtractEquipmentOllama(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	if gearText == "" {
		return nil, nil
	}

	reqBody := OllamaRequest{
        Model: ollamaModel(),
        Messages: []Message{
            {Role: "system", Content: equipmentSystemPrompt()},
            {Role: "user", Content: equipmentUserPrompt(gearText)},
        },
        Stream: false,
        Options: map[string]any{
            "num_predict": 512,
            "temperature": 0,   // greedy, deterministic JSON
        },
    }
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL()+"/api/chat", bytes.NewReader(body))
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
    var ollamaResp OllamaResponse
    if err := json.Unmarshal(raw, &ollamaResp); err != nil {
        return nil, err
    }
	
	var items []EquipmentItem
	if err := json.Unmarshal([]byte(ollamaResp.Message.Content), &items); err != nil {
		return nil, fmt.Errorf("parsing equipment JSON: %w", err)
	}

	return items, nil
}

func ExtractEquipmentGemini(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	// The client gets the API key from the environment variable `GEMINI_API_KEY`.
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("creating Gemini client: %w", err)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		geminiModel(),
		genai.Text(equipmentSystemPrompt()+"\n\n"+equipmentUserPrompt(gearText)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("calling Gemini API: %w", err)
	}

	resultStr := jsonArrayRe.FindString(result.Text())
	if resultStr == "" {
		return nil, fmt.Errorf("no JSON array found in Gemini response")
	}

	var items []EquipmentItem
	if err := json.Unmarshal([]byte(resultStr), &items); err != nil {
		return nil, fmt.Errorf("parsing equipment JSON: %w", err)
	}

	return items, nil
}
