package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
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

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

// ExtractEquipment parses a free-form gear description into a structured list.
// lang is "fr" or "en" and drives the prompt language.
// Returns an error if Ollama is unreachable or returns unparseable output.
func ExtractEquipment(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	if gearText == "" {
		return nil, nil
	}

	langName := "French"
	if lang == "en" {
		langName = "English"
	}

	prompt := fmt.Sprintf(`You are a mountain climbing equipment assistant.	Parse the following gear description and return a JSON array.
	Each element must have exactly three fields:
	"name": equipment name (string, in %s)
	"quantity": number needed (integer, 1 if unspecified)
	"notes": "optional" or "mandatory", plus any relevant detail (string, in %s)
	The name of these equipments are related with the moutain climbing activity. They're also personal equipment that climbers should bring, for instance a relay is not one of them, quickdraws or rope are.
	You should include only equipment you're absolutely sure about.
	Output ONLY the JSON array, no explanation. 
	Gear description:%s`, langName, langName, gearText)

	body, _ := json.Marshal(map[string]any{
		"model":  ollamaModel(),
		"prompt": prompt,
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaURL()+"/api/generate", bytes.NewReader(body))
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

	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}

	match := jsonArrayRe.FindString(ollamaResp.Response)
	if match == "" {
		return nil, fmt.Errorf("no JSON array found in LLM response")
	}

	var items []EquipmentItem
	if err := json.Unmarshal([]byte(match), &items); err != nil {
		return nil, fmt.Errorf("parsing equipment JSON: %w", err)
	}

	return items, nil
}
