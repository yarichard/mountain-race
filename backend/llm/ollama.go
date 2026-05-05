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
)

type ollamaProvider struct{}

func (p *ollamaProvider) ExtractEquipment(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	return ExtractEquipmentOllama(ctx, gearText, lang)
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

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
}

// ExtractEquipmentOllama is kept as a package-level function so existing tests compile unchanged.
func ExtractEquipmentOllama(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	if gearText == "" {
		return nil, nil
	}

	// Ollama inference can take minutes — detach from the HTTP request context
	// and use a hard deadline instead.
	ollamaCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	reqBody := ollamaChatRequest{
		Model: ollamaModel(),
		Messages: []ollamaChatMessage{
			{Role: "system", Content: equipmentSystemPrompt()},
			{Role: "user", Content: equipmentUserPrompt(gearText)},
		},
		Stream: false,
		Options: map[string]any{
			"num_predict": 512,
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

	match := jsonArrayRe.FindString(ollamaResp.Message.Content)
	if match == "" {
		return nil, fmt.Errorf("no JSON array found in ollama response")
	}

	var items []EquipmentItem
	if err := json.Unmarshal([]byte(match), &items); err != nil {
		return nil, fmt.Errorf("parsing equipment JSON: %w", err)
	}

	return items, nil
}
