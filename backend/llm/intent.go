package llm

import (
	"fmt"
	"regexp"
	"time"
)

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

// jsonObjectRe extracts the first {...} block from an LLM response.
var jsonObjectRe = regexp.MustCompile(`(?s)\{.*\}`)

func intentSystemPrompt(lang string) string {
	year := time.Now().Year()
	if lang == "en" {
		return fmt.Sprintf(`You are a mountain race planning assistant. Parse the user's description of a mountain race and extract structured parameters.

Return ONLY a JSON object with exactly two top-level keys:
- "intent": an object with the fields you could confidently extract
- "missing": an array of required field names you could NOT determine

The "intent" object may contain:
- "location": place name or GPS coords (string)
- "location_type": by default use "location" only use "name" if user request explicitly for a route name (string)
- "race_type": one of "multipitch", "ridge_hike", "hike" (string)
- "difficulty": French sport grade (e.g. "5c") for multipitch; alpine cotation (F, PD, AD, D, TD, ED) for hikes/ridges (string)
- "date": race date in YYYY-MM-DD format; current year is %d unless the user specifies otherwise (string)
- "participants": array of {"name": string, "climbing_level": string} objects

Required fields: "location", "location_type", "race_type", "date".
Optional fields: "difficulty", "participants".
Only include a field in "intent" if you are confident about its value.
List in "missing" any required fields absent from the description.
Output ONLY the JSON object, no explanation.`, year)
	}
	return fmt.Sprintf(`Tu es un assistant de planification de courses en montagne. Analyse la description de l'utilisateur et extrais les paramètres structurés.

Retourne UNIQUEMENT un objet JSON avec exactement deux clés de premier niveau :
- "intent" : un objet avec les champs que tu as pu extraire avec certitude
- "missing" : un tableau des noms des champs obligatoires que tu N'AS PAS pu déterminer

L'objet "intent" peut contenir :
- "location" : nom de lieu ou coordonnées GPS (string)
- "location_type" : "location" par défaut, utilise seulement"name" si l'utilisateur demande explicitement un nom de route (string)
- "race_type" : une des valeurs "multipitch", "ridge_hike", "hike" (string)
- "difficulty" : cotation sport française (ex : "5c") pour multipitch ; cotation alpine (F, PD, AD, D, TD, ED) pour randonnées/arêtes (string)
- "date" : date de la course au format YYYY-MM-DD ; l'année en cours est %d sauf indication contraire (string)
- "participants" : tableau d'objets {"name": string, "climbing_level": string}

Champs obligatoires : "location", "location_type", "race_type", "date".
Champs optionnels : "difficulty", "participants".
N'inclure dans "intent" que les champs dont tu es certain.
Lister dans "missing" les champs obligatoires absents de la description.
Retourner UNIQUEMENT l'objet JSON, sans explication.`, year)
}

func intentUserPrompt(text string) string {
	return "Race description:\n" + text
}
