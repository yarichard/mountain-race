package llm

import "regexp"

// EquipmentItem is the structured output from LLM gear parsing.
type EquipmentItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
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
