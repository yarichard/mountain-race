package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// EquipmentItem is the structured output from LLM gear parsing.
type EquipmentItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Notes    string `json:"notes"`
}

var jsonArrayRe = regexp.MustCompile(`(?s)\[.*\]`)

// jsonObjectRe extracts the first {...} block from an LLM response.
var jsonObjectRe = regexp.MustCompile(`(?s)\{.*\}`)


// RaceIntent holds the structured fields extracted from natural-language input.
type RaceIntent struct {
	Location     string              `json:"location,omitempty"`
	LocationType string              `json:"location_type,omitempty"`
	RaceType     string              `json:"race_type,omitempty"`
	Difficulty   string              `json:"difficulty,omitempty"`
	Date         string              `json:"date,omitempty"`
	Participants []IntentParticipant `json:"participants,omitempty"`
}

// IntentParticipant is a participant parsed from natural-language intent.
type IntentParticipant struct {
	Name          string `json:"name"`
	ClimbingLevel string `json:"climbing_level"`
}

// ParseIntentResult is the structured response from the intent LLM call.
type ParseIntentResult struct {
	Intent  RaceIntent `json:"intent"`
	Missing []string   `json:"missing"`
}

// requiredIntentFields is the allowlist for the "missing" array.
// Small models sometimes include optional field names; we strip those out.
var requiredIntentFields = map[string]bool{
	"location": true, "location_type": true, "race_type": true, "date": true,
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
	filtered := result.Missing[:0]
	for _, f := range result.Missing {
		if requiredIntentFields[f] {
			filtered = append(filtered, f)
		}
	}
	result.Missing = filtered
	return &result, nil
}

func intentSystemPrompt(lang string) string {
	year := time.Now().Year()
	if lang == "en" {
		return fmt.Sprintf(`You are a mountain race planning assistant. Reply ONLY with a valid JSON object. No text before or after. The "missing" key is always present.

Format: {"intent": {FIELDS}, "missing": [ABSENT_REQUIRED_FIELDS]}

Required fields (1–4) — add to "missing" if absent:
1. "location": geographic place (mountain, valley, town) or full route/ridge name if explicitly named (string)
2. "location_type": "name" if user explicitly names a route or ridge (e.g. "Rébuffat route", "Devil's Ridge"), otherwise "location" (string)
3. "race_type": "multipitch"=rock climbing/big wall; "ridge_hike"=ridge/traverse; "hike"=hiking/trail (string)
4. "date": YYYY-MM-DD, default year %d. "30/06"→"%d-06-30", "Aug 15"→"%d-08-15" (string)

Optional fields (5–6) — include if present, NEVER put in "missing":
5. "difficulty": climbing grade e.g. "5c","6a" or alpine grade "F","PD","AD","D","TD","ED"
6. "participants": [{"name":"...","climbing_level":"..."}]

Examples:
- "big wall climb near Argis on 30/06" → {"intent":{"location":"Argis","location_type":"location","race_type":"multipitch","date":"%d-06-30"},"missing":[]}
- "Rébuffat route July 20 AD" → {"intent":{"location":"Rébuffat route","location_type":"name","race_type":"multipitch","date":"%d-07-20","difficulty":"AD"},"missing":[]}
- "hike to Mont Blanc Aug 15 with Alice 5a and Bob 6b" → {"intent":{"location":"Mont Blanc","location_type":"location","race_type":"hike","date":"%d-08-15","participants":[{"name":"Alice","climbing_level":"5a"},{"name":"Bob","climbing_level":"6b"}]},"missing":[]}`,
			year, year, year, year, year, year)
	}
	return fmt.Sprintf(`Tu es un assistant de planification de courses en montagne. Réponds UNIQUEMENT avec un objet JSON valide. Aucun texte avant ou après. La clé "missing" est toujours présente.

Format : {"intent": {CHAMPS}, "missing": [CHAMPS_OBLIGATOIRES_ABSENTS]}

Champs obligatoires (1–4) — mettre dans "missing" si absents :
1. "location" : lieu géographique (sommet, vallée, ville) ou nom complet de la voie/arête si explicitement nommée (string)
2. "location_type" : "name" si l'utilisateur cite un nom de voie/arête précis (ex: "voie Rébuffat", "arête du Diable"), sinon "location" (string)
3. "race_type" : "multipitch"=grande voie/escalade ; "ridge_hike"=arête/traversée ; "hike"=randonnée (string)
4. "date" : YYYY-MM-DD, année %d par défaut. "30/06"→"%d-06-30", "15 août"→"%d-08-15", "12/08"→"%d-08-12" (string)

Champs optionnels (5–6) — inclure si présents, JAMAIS dans "missing" :
5. "difficulty" : cotation escalade "5c","6a" ou alpine "F","PD","AD","D","TD","ED"
6. "participants" : [{"name":"...","climbing_level":"..."}]

Exemples :
- "grande voie à Argis le 30/06" → {"intent":{"location":"Argis","location_type":"location","race_type":"multipitch","date":"%d-06-30"},"missing":[]}
- "voie Rébuffat le 20 juillet AD" → {"intent":{"location":"voie Rébuffat","location_type":"name","race_type":"multipitch","date":"%d-07-20","difficulty":"AD"},"missing":[]}
- "arête des Cosmiques le 5/07 avec Jean 5b et Sophie 6a" → {"intent":{"location":"arête des Cosmiques","location_type":"name","race_type":"ridge_hike","date":"%d-07-05","participants":[{"name":"Jean","climbing_level":"5b"},{"name":"Sophie","climbing_level":"6a"}]},"missing":[]}`,
		year, year, year, year, year, year, year)
}

func intentUserPrompt(text, lang string) string {
	if lang == "en" {
		return "Race description:\n" + text
	}
	return "Description de la course :\n" + text
}

func equipmentSystemPrompt() string {
	return `Tu es un assistant spécialisé en matériel d'alpinisme et d'escalade.
À partir d'une description de matériel, retourne un tableau JSON. Chaque élément a exactement trois champs :
- "name": nom du matériel (string, en français)
- "quantity": quantité nécessaire (integer, 1 si non précisé)
- "notes": "obligatoire" ou "facultatif", suivi d'un détail court si utile (string, en français)

Exemple d'entrée :
14 dégaines. Un jeu de friends du 0.3 au 2. Corde à double conseillée (2x50m).

Exemple de sortie :
[{"name":"Dégaines","quantity":14,"notes":"obligatoire"},{"name":"Friend #0.3","quantity":1,"notes":"obligatoire"},{"name":"Friend #0.5","quantity":1,"notes":"obligatoire"},{"name":"Friend #1","quantity":1,"notes":"obligatoire"},{"name":"Friend #2","quantity":1,"notes":"obligatoire"},{"name":"Corde à double","quantity":2,"notes":"facultatif, 50m chacune"}]

Règles :
- N'inclure que le matériel technique (corde, dégaines, friends, sangles, crampons, piolet, etc.)
- Ne pas inclure les vêtements, la nourriture, ni les chaussures
- Si la description est vide ou ne contient aucun matériel identifiable, retourne []
- Réponds UNIQUEMENT avec le tableau JSON, sans texte autour, sans balises markdown`
}

func equipmentUserPrompt(gearText string) string {
	return "Description du matériel :\n" + gearText
}

// DurationStep is one named stage found in the description.
type DurationStep struct {
	Label string  `json:"label"`
	Hours float64 `json:"hours"`
}

// DurationResult is the structured output from LLM duration parsing.
type DurationResult struct {
	TotalHours float64        `json:"total_hours"`
	Confidence string         `json:"confidence"`
	Steps      []DurationStep `json:"steps"`
}

func parseDurationJSON(raw string) (*DurationResult, error) {
	match := jsonObjectRe.FindString(raw)
	if match == "" {
		return nil, fmt.Errorf("no JSON object found in LLM response")
	}
	var result DurationResult
	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return nil, fmt.Errorf("parsing duration JSON: %w", err)
	}
	return &result, nil
}

func durationSystemPrompt(lang string) string {
	if lang == "en" {
		return `You are a mountain route planning assistant. Read the route description and extract timing information.
Return a JSON object with exactly these fields:
- "total_hours": total estimated duration in hours as a float (0 if not found)
- "confidence": "high" if a clear total is stated, "medium" if computed from steps, "low" if estimated
- "steps": array of {"label": "stage name", "hours": N} for any named stages/pitches/sections that mention a duration

Reply ONLY with a valid JSON object. No text before or after.

Example: {"total_hours": 7.5, "confidence": "medium", "steps": [{"label": "Approach", "hours": 1.5}, {"label": "Climb", "hours": 4}, {"label": "Descent", "hours": 2}]}`
	}
	return `Tu es un assistant de planification de courses en montagne. Lis la description de la course et extrais les informations de durée.
Retourne un objet JSON avec exactement ces champs :
- "total_hours" : durée totale estimée en heures (float), 0 si introuvable
- "confidence" : "high" si une durée totale est clairement indiquée, "medium" si calculée depuis des étapes, "low" si estimée
- "steps" : tableau de {"label": "nom de l'étape", "hours": N} pour chaque étape/longueur/section qui mentionne une durée

Réponds UNIQUEMENT avec un objet JSON valide. Aucun texte avant ou après.

Exemple : {"total_hours": 7.5, "confidence": "medium", "steps": [{"label": "Approche", "hours": 1.5}, {"label": "Escalade", "hours": 4}, {"label": "Descente", "hours": 2}]}`
}

func durationUserPrompt(description, lang string) string {
	if lang == "en" {
		return "Route description:\n" + description
	}
	return "Description de la course :\n" + description
}
