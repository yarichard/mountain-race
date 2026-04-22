# LLM Equipment Extraction

## Goal

Replace the naive `parseEquipment` function in `backend/camptocamp/detail.go` — which currently dumps the raw C2C gear text as a single Equipment row — with an LLM call that parses the text into a structured list of `{item, quantity, notes}` tuples.

---

## Context

- `parseEquipment` reads the `gear` locale field from the C2C API response (free-form text, French or English depending on `lang`).
- The result feeds `RouteDetail.Equipment []Equipment`, consumed by the frontend `EquipmentPanel`.
- `Equipment` struct: `Item string`, `Quantity int`, `Notes string`.
- Ollama runs on the host at `http://host.docker.internal:11434` (accessible from inside the container).

---

## Architecture

### New package: `backend/llm`

Single file `backend/llm/equipment.go` exposing:

```go
// ExtractEquipment parses a free-form gear description into a structured list.
// lang is "fr" or "en" and drives prompt language.
// Returns an error if Ollama is unreachable or returns unparseable output.
func ExtractEquipment(ctx context.Context, gearText, lang string) ([]camptocamp.Equipment, error)
```

### Configuration (env vars)

| Var | Default | Purpose |
|-----|---------|---------|
| `OLLAMA_URL` | `http://host.docker.internal:11434` | Ollama host |
| `OLLAMA_MODEL` | `llama3.2` | Model name |

Read once at startup (or lazily on first call) via `os.Getenv`.

### Ollama Go client

Use the official `github.com/ollama/ollama/api` package.
- Call `client.Generate` (or `client.Chat`) with `stream: false`.
- Request JSON output via prompt instructions (Ollama does not yet have reliable structured-output for all models, so we instruct via prompt and parse the JSON ourselves).

---

## Prompt Design

The prompt instructs the model to output **only** a JSON array, no prose:

```
You are a mountain climbing equipment assistant.
Parse the following gear description and return a JSON array.
Each element must have exactly three fields:
  "name": equipment name (string, in {LANG})
  "quantity": number needed (integer, 1 if unspecified)
  "notes": "optional" or "mandatory", plus any relevant detail (string, in {LANG})

Output ONLY the JSON array, no explanation.

Gear description:
{GEAR_TEXT}
```

- `{LANG}` is replaced with `"French"` or `"English"`.
- `{GEAR_TEXT}` is the raw C2C gear text.

### JSON extraction

After receiving the response, extract the first `[...]` block with a simple regex (`\[[\s\S]*\]`) and unmarshal into a temporary struct:

```go
type llmEquipmentItem struct {
    Name     string `json:"name"`
    Quantity int    `json:"quantity"`
    Notes    string `json:"notes"`
}
```

Map to `camptocamp.Equipment{Item: ..., Quantity: ..., Notes: ...}`.

---

## Integration point

In `backend/camptocamp/detail.go`, replace the call to `parseEquipment` in `GetDetail`:

```go
equipment, err := llm.ExtractEquipment(ctx, gearText, lang)
if err != nil {
    return nil, fmt.Errorf("equipment parsing failed: %w", err)
}
```

`GetDetail` signature gains a `ctx context.Context` parameter (already a good practice for HTTP calls).

The raw gear text extraction stays in `detail.go` as a helper `extractGearText(m, lang) string`; only the structuring moves to the `llm` package.

---

## Error handling

- Ollama unreachable → return error, `GetDetail` returns error, API returns `500` with message `"equipment parsing failed: ..."`.
- LLM returns non-parseable JSON → return error (same path).
- Empty gear text → skip LLM call, return empty slice (no error).

---

## Files to create / modify

| File | Action | What changes |
|------|--------|--------------|
| `backend/llm/equipment.go` | **Create** | `ExtractEquipment` function |
| `backend/llm/equipment_test.go` | **Create** | Unit test with a mock HTTP server standing in for Ollama |
| `backend/camptocamp/detail.go` | **Modify** | `GetDetail` gains `ctx`, calls `llm.ExtractEquipment`, `parseEquipment` replaced by `extractGearText` |
| `backend/camptocamp/detail_test.go` | **Modify** | Adjust for new `GetDetail` signature; add test for empty gear text path |
| `backend/api/routes.go` | **Modify** | Pass `context` from Gin handler into `GetDetail` |
| `backend/go.mod` / `go.sum` | **Modify** | Add `github.com/ollama/ollama/api` dependency |
| `.env.example` | **Modify** | Document `OLLAMA_URL`, `OLLAMA_MODEL` |

---

## Build sequence

1. Add `github.com/ollama/ollama/api` to `go.mod` (`go get`).
2. Create `backend/llm/equipment.go`.
3. Update `detail.go` — extract gear text, call `llm.ExtractEquipment`.
4. Update `api/routes.go` to thread `ctx`.
5. Write unit tests for `llm` package (mock Ollama server).
6. Update `.env.example`.
7. Verify `make build` compiles; run `go test ./...` in backend.
