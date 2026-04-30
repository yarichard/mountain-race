# Mountain Race

A single-page application to help plan mountain races with a group of friends. Enter your team, pick a route type and difficulty, search CampToCamp for matching routes, and get a full race plan — including weather, avalanche risk, schedule, equipment list, and a PDF export.

![Screen](./Screenshot.png)
---

## Features

- **Route search** — searches [CampToCamp](https://www.camptocamp.org) by route name or geographic area (geocoded via OpenStreetMap Nominatim)
- **Route detail** — topo/description, interactive map (real GPS from C2C), elevation profile, pitch-by-pitch breakdown for multi-pitch climbs
- **Weather & avalanche** — MeteoFrance AROME/ARPEGE forecast + DPBRA avalanche bulletin
- **Schedule** — estimated duration from CampToCamp data or Naismith's rule fallback
- **Equipment list** — sourced from CampToCamp gear data
- **PDF export** — full race plan exported as landscape A4 via headless Chromium
- **Bilingual** — French and English, auto-detected from the browser

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Docker Container (port 8003)                   │
│                                                 │
│  Gin (Go)                                       │
│  ├── /api/*          REST endpoints             │
│  └── /*              Static file serving        │
│                      (Next.js static export)    │
└─────────────────────────────────────────────────┘
```

| Layer | Technology |
|---|---|
| Frontend | Next.js 16 · TypeScript · Tailwind CSS · next-intl |
| Backend | Go 1.26 · Gin · chromedp |
| Maps | Leaflet / react-leaflet |
| Charts | Recharts |
| Geocoding | OpenStreetMap Nominatim (no key required) |
| Route data | CampToCamp anonymous read-only API |
| Weather | MeteoFrance AROME / ARPEGE / DPBRA APIs |

---

## Prerequisites

**To run with Docker** (recommended):
- Docker

**To run locally** (development):
- Go 1.26+
- Node.js 24+ and npm

Nothing is installed on the host machine when using the devcontainer — see [Devcontainer](#devcontainer).

---

## Quick Start

### 1. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and fill in your MeteoFrance credentials:

```
METEOFRANCE_USER=your_username
METEOFRANCE_PASS=your_password
```

> MeteoFrance credentials are required for real weather data. Without them the weather panel shows mock data.

### 2. Build and run with Docker

```bash
make build   # builds the Docker image
make run     # runs on http://localhost:8003
```

Open [http://localhost:8003](http://localhost:8003).

---

## Development

### Local build (no Docker)

```bash
make local-build   # builds frontend, copies static files, compiles Go binary
make local-run     # starts the server on http://localhost:8003
```

`local-build` does three things in sequence:
1. `cd frontend && npm ci && npm run build` → produces `frontend/out/`
2. Copies `frontend/out/` → `backend/static/`
3. `cd backend && go build -o server .`

### Backend only (API development)

```bash
cd backend
go run .
```

The server starts on port 8003. Without a built frontend it returns a JSON status message for non-API routes.

### Frontend only (UI development)

```bash
cd frontend
npm install
npm run dev    # starts Next.js dev server on http://localhost:3000
```

> In dev mode, `/api/*` calls won't reach the Go backend. Run the backend separately or point to a running container.

### Running tests

```bash
# Backend unit tests
cd backend
go test ./...

# Verbose output
go test ./camptocamp/ -v
```

---

## Project Structure

```
mountain-race/
├── frontend/                   # Next.js TypeScript project (static export)
│   ├── src/
│   │   ├── app/                # Next.js App Router (single page)
│   │   ├── components/         # One component per panel (9 panels)
│   │   └── lib/                # Types, i18n helpers
│   └── messages/               # fr.json, en.json translation files
├── backend/                    # Go / Gin project
│   ├── main.go                 # Entry point: .env loading, router, static serving
│   ├── api/                    # HTTP handlers
│   ├── camptocamp/             # CampToCamp API client + unit tests
│   ├── meteo/                  # MeteoFrance token, forecast, avalanche
│   ├── schedule/               # Naismith's rule
│   └── pdf/                    # headless Chromium PDF export
├── planning/
│   └── plan.md                 # Build plan and implementation status
├── Makefile
├── Dockerfile                  # Multi-stage production build
├── Dockerfile.devcontainer     # Devcontainer for VS Code
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## API Reference

All endpoints are prefixed with `/api`.

### `POST /api/routes/search`

Search CampToCamp for routes matching the criteria.

```json
{
  "location": "Chamonix",
  "location_type": "location",
  "race_type": "multipitch",
  "difficulty": "5c",
  "date": "2026-07-15",
  "participants": [
    { "name": "Alice", "climbing_level": "6a" }
  ]
}
```

- `location_type`: `"name"` searches by route name (C2C `q` param); `"location"` geocodes the text and searches by bounding box (20 km radius)
- `location`: free text, place name, or `"lat,lon"` (the `lat,lon` format bypasses Nominatim for `"location"` type)
- `difficulty`: French sport grade (`5c`, `6a+`, …) for `multipitch`; alpine cotation (`F`, `PD`, `AD`, `D`, `TD`, `ED`) for hikes

**Response `200`:**
```json
{
  "routes": [
    {
      "id": "985727",
      "title": "Voie du Peigne",
      "summary": "...",
      "difficulty": "5c",
      "elevation_gain": 650,
      "distance_km": 4.2,
      "source_url": "https://www.camptocamp.org/routes/985727"
    }
  ]
}
```

---

### `GET /api/routes/:id`

Full detail for a CampToCamp route.

**Response `200`:**
```json
{
  "id": "985727",
  "title": "Voie du Peigne",
  "description": "...",
  "difficulty": "5c",
  "elevation_gain": 650,
  "distance_km": 4.2,
  "lat": 45.92,
  "lon": 6.87,
  "pitches": [
    { "number": 1, "grade": "5b", "description": "Dalle initiale" }
  ],
  "topo_url": "https://media.camptocamp.org/...",
  "gpx_url": "",
  "equipment": [
    { "item": "Corde 60m", "quantity": 1, "notes": "Simple" }
  ],
  "risks": ["Chutes de pierres en début de journée"],
  "alternative_routes": [
    { "id": "234567", "title": "Arête des Cosmiques", "reason": "Itinéraire alternatif" }
  ],
  "schedule": {
    "estimated_duration_hours": 5.5,
    "recommended_start_time": "06:00",
    "recommended_end_time": "14:00",
    "source": "camptocamp"
  },
  "source_url": "https://www.camptocamp.org/routes/985727"
}
```

`schedule.source` is `"camptocamp"` when duration data exists on C2C, or `"formula"` when computed via Naismith's rule. The UI shows a notice in the formula case.

---

### `GET /api/weather?lat=45.9&lon=6.9&date=2026-07-15`

Weather forecast and avalanche risk for a location and date.

**Response `200`:**
```json
{
  "forecast": {
    "date": "2026-07-15",
    "temperature_min_c": 8.0,
    "temperature_max_c": 22.0,
    "precipitation_mm": 0.0,
    "wind_speed_kmh": 15.0,
    "condition": "sunny"
  },
  "avalanche": {
    "risk_level": 2,
    "risk_label": "Limité",
    "description": "Risque faible en altitude..."
  }
}
```

Uses MeteoFrance **AROME** (≤ 48 h) or **ARPEGE** (> 48 h) for the forecast, and **DPBRA** for the avalanche bulletin.

---

### `POST /api/export/pdf`

Generate a PDF of the full race plan. Request body is the `GET /api/routes/:id` response shape plus a `weather` block. Returns `application/pdf`.

---

## Environment Variables

| Variable | Description | Required |
|---|---|---|
| `METEOFRANCE_USER` | MeteoFrance API username | For real weather data |
| `METEOFRANCE_PASS` | MeteoFrance API password | For real weather data |

The backend loads `.env` from the project root (or the parent of the binary directory inside Docker).

---

## Devcontainer

The project uses a VS Code devcontainer defined in `Dockerfile.devcontainer`. It provides Go, Node.js, and all required system libraries (`libeccodes-dev`, etc.) without installing anything on the host machine.

To use it: open the project in VS Code and select **Reopen in Container** when prompted.

> Docker is not available inside the devcontainer. Use `make local-build` + `make local-run` for development, or build/run from the host machine.

---

## Known Limitations

| Area | Status |
|---|---|
| **Weather forecast (GRIB2)** | `meteo/forecast.go` is a stub returning mock data. Real GRIB2 decoding via `github.com/meteocima/eccodes-go` is not yet implemented. `libeccodes-dev` is installed in the Docker image and ready. |
| **Weather uses route GPS** | The frontend currently passes hardcoded coordinates (`lat=45.9&lon=6.9`) to `/api/weather`. It should use the route's real `lat`/`lon` from the detail response. |
| **Elevation profile** | The chart shows a synthetic bell-curve based on elevation gain and distance. Real GPX track decoding is not yet implemented. |
| **Nominatim rate limit** | The geocoding endpoint is subject to Nominatim's public 1 req/s limit. Production use should add caching or a self-hosted instance. |
| **Frontend unit tests** | React Testing Library tests are planned but not yet written. |
| **E2E tests** | Playwright test suite (`test/`) is planned but not yet implemented. |

## Training gear equipment model
- Generate jsonl file using **generate_gear_dataset go helper
- Clean & prepare data using gear_preparing.ipynb locally
- Train model using gear_training.ipynb on Google collab
- Test the model using gear_testing.ipynb on Google collab
- (Optional) For ollama local use:
  - Download the trained & merged model `hf download yrichard/gear_training-2026-04-28_13.15.01-merged --local-dir ./data/gear_merged`. Be careful, model needs not to be loaded with quantization in order to make it work
  - Convert the model to Ollama gguf format `python ../llama.cpp/convert_hf_to_gguf.py ./gear_merged --outfile ./data/gear.gguf --outtype q8_0` (using llama.cpp `git clone https://github.com/ggerganov/llama.cpp.git`)
  - Import the model into Ollama: `ollama create gear-assistant -f Modelfile`

  ![Training](./data/wandb_training.png)