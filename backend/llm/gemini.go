package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"google.golang.org/genai"
)

type geminiProvider struct{}

func (p *geminiProvider) ExtractEquipment(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	return ExtractEquipmentGemini(ctx, gearText, lang)
}

func geminiModel() string {
	if m := os.Getenv("GEMINI_MODEL"); m != "" {
		return m
	}
	return "gemini-2.5-flash-lite"
}

func ExtractEquipmentGemini(ctx context.Context, gearText, lang string) ([]EquipmentItem, error) {
	if gearText == "" {
		return nil, nil
	}

	// The client reads GEMINI_API_KEY from the environment.
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
