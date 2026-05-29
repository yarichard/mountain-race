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
