# Natural Language Race Intent Feature

## Overview

Allow users to describe their desired race in plain text inside Part 4 (Race Search). The backend parses the text with an LLM, extracts structured parameters, and auto-fills the form + triggers the route search. If required fields are missing, the UI prompts the user to complete them before searching.

---

## User Flow

1. User types a free-form description in a text area at the top of Part 4 (e.g. "Une course de crête niveau AD avec 3 amis autour de Chamonix le 15 juillet").
2. User clicks **"Décrire ma course"** / **"Describe my race"**.
3. Frontend sends `POST /api/intent/parse` with the text and browser language.
4. Backend LLM extracts race parameters and returns:
   - `intent`: structured fields it could confidently extract
   - `missing`: list of required field names the LLM could not determine
5. Frontend auto-fills the Part 4 form fields from `intent`.
6. If `missing` is non-empty → highlight those fields inline, wait for user to complete them.
7. Once all required fields are present → auto-trigger `POST /api/routes/search` (same path as manual search).

---

## Required vs Optional Fields

| Field | Required | Notes |
|-------|----------|-------|
| `location` | Yes | Place name or GPS coords |
| `location_type` | Yes | Inferred from location format (`name` or `location`) |
| `race_type` | Yes | `multipitch`, `ridge_hike`, or `hike` |
| `date` | Yes | YYYY-MM-DD |
| `difficulty` | No | French sport grade or alpine cotation |
| `participants` | No | Name + climbing level per person |

---

## New API Endpoint

### `POST /api/intent/parse`

**Request body:**
```json
{
  "text": "string",   // free-form user description
  "lang": "fr|en"     // drives LLM prompt language
}
```

**Response body:**
```json
{
  "intent": {
    "location": "string",
    "location_type": "name|location",
    "race_type": "multipitch|ridge_hike|hike",
    "difficulty": "string",
    "date": "2006-01-02",
    "participants": [
      { "name": "string", "climbing_level": "string" }
    ]
  },
  "missing": ["date", "race_type"]   // list of required fields not found; empty = ready to search
}
```

---

## Backend Implementation

### Files to create

#### `backend/llm/intent.go`
- `RaceIntent` struct (mirrors `routes.SearchRequest` fields).
- `ParseRaceIntent(ctx context.Context, text, lang string) (*RaceIntent, []string, error)` — calls the intent LLM provider.
- System prompt instructs the LLM to:
  - Extract the 6 fields from the table above.
  - Return a JSON object with only fields it is confident about (omit unknowns).
  - Normalize dates to YYYY-MM-DD (relative dates like "le 15 juillet" → next occurrence).
  - Normalize `race_type` to one of the three enum values.
  - Normalize `difficulty` to the correct scale based on detected `race_type`.
  - Respond in the language specified by `lang`.

#### `backend/llm/intent_provider.go`
- `intentProvider` interface (same shape as equipment `Provider`).
- `NewIntentProvider()` factory reads `INTENT_LLM_PROVIDER` env var; falls back to Gemini.
- Configurable via `INTENT_LLM_MODEL` env var.
- Reuses `openai.go` / `gemini.go` / `ollama.go` transport; only prompt differs.

### Files to modify

#### `backend/api/intent.go` (new handler file)
- `ParseIntentHandler(c *gin.Context)` — reads body, calls `llm.NewIntentProvider().ParseRaceIntent(...)`, returns JSON.

#### `backend/main.go` (or router file)
- Register `POST /api/intent/parse` route pointing to `ParseIntentHandler`.

### Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `INTENT_LLM_PROVIDER` | `gemini` | Provider for intent parsing (`gemini`, `openai`, `ollama`) |
| `INTENT_LLM_MODEL` | provider default | Model override for intent parsing |

---

## Frontend Implementation

### Files to modify

#### Part 4 component (Race Search section)
- Add a `<textarea>` + submit button at the **top** of the section, above the existing structured fields.
- On submit: call `POST /api/intent/parse`, set loading state.
- On success: merge returned `intent` fields into form state (do not overwrite fields the LLM left empty).
- Highlight fields listed in `missing` with a visual indicator (e.g. orange border + tooltip).
- Watch form state: when all required fields become non-empty, auto-trigger the existing search handler.

#### i18n strings (fr + en)
- Text area placeholder: "Décrivez votre course..." / "Describe your race..."
- Button label: "Analyser" / "Analyze"
- Missing field hint: "Champ requis" / "Required field"

---

## Testing

### Backend unit tests (`backend/llm/intent_test.go`)
- Mock LLM response → assert correct `RaceIntent` parsing.
- Missing required fields → assert correct `missing` slice.
- Ambiguous `race_type` → assert graceful omission.

### Backend API tests (`backend/api/intent_test.go`)
- `POST /api/intent/parse` with valid body → 200 + intent.
- Empty text → 400.
- LLM error → 500.

### Frontend tests
- Textarea renders inside Part 4.
- Submitting fills form fields.
- Missing fields are highlighted.
- Auto-search triggers once required fields are populated.

---

## Implementation Order

1. `backend/llm/intent.go` — struct + prompt + parsing logic
2. `backend/llm/intent_provider.go` — factory + provider wiring
3. `backend/api/intent.go` — HTTP handler
4. Register route in router
5. Frontend: textarea + parse call + form merge
6. Frontend: missing-field highlights + auto-search trigger
7. Unit + API tests
8. Update `.env.example` with new env vars
