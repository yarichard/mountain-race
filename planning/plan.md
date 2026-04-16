# Mountain Race — Build Plan

Reference spec: `../CLAUDE.md`

All steps must be validated inside the Docker container (port 8003). Nothing is installed on the host machine; work happens inside the devcontainer.

---

## Implementation Status

| Phase | Description | Status |
|---|---|---|
| 1 | Project scaffolding | ✅ Done |
| 2 | Backend | ✅ Done (weather GRIB2 pending — see note) |
| 3 | Frontend | ✅ Done |
| 4 | Integration & Docker build | ✅ Done |
| 5 | E2E tests | ⬜ Not started |
| 6 | Final checks | 🔄 Partial |

---

## Phase 1 — Project Scaffolding ✅

### Step 1.1 — Root files ✅
- `.gitignore` — ignores `node_modules/`, `frontend/.next/`, `frontend/out/`, `backend/bin/`, `.env`
- `.env.example` — documents `METEOFRANCE_USER` and `METEOFRANCE_PASS`
- `docker-compose.yml` — convenience wrapper around the production Dockerfile on port 8003
- `Makefile` with four targets:
  - `build`: `docker build -t mountain-race .`
  - `run`: `docker run --env-file .env -p 8003:8003 mountain-race`
  - `local-build`: build frontend → copy static → build Go binary (no Docker required)
  - `local-run`: run the Go binary locally on port 8003

### Step 1.2 — Production Dockerfile ✅
Multi-stage build:
```
Stage 1: golang:1.26.2
  - apt-get install libeccodes-dev
  - CGO_ENABLED=1 go build → /app/server

Stage 2: node:24-slim
  - apt-get install chromium libeccodes-dev ca-certificates
  - npm ci && npm run build → frontend/out/
  - Copy /app/server from Stage 1
  - Copy frontend/out/ → /app/static/
  - EXPOSE 8003
  - CMD ["./server"]
```

---

## Phase 2 — Backend ✅

### Step 2.1 — Go module ✅
`go mod init mountain-race` in `backend/`. Dependencies:
- `github.com/gin-gonic/gin`
- `github.com/joho/godotenv`
- `github.com/chromedp/chromedp`

> **Note**: `github.com/meteocima/eccodes-go` was intentionally excluded — GRIB2 decoding is not yet implemented (see Step 2.4).

### Step 2.2 — Project structure ✅
```
backend/
├── main.go
├── api/
│   ├── register.go
│   ├── routes.go         # POST /api/routes/search, GET /api/routes/:id
│   ├── weather.go        # GET /api/weather
│   └── export.go         # POST /api/export/pdf
├── camptocamp/
│   ├── client.go         # HTTP client; baseURL is a var (injectable in tests)
│   ├── search.go         # name search (q param) + location search (geocode → bbox)
│   └── detail.go         # full route detail: pitches, equipment, risks, schedule, lat/lon
├── meteo/
│   ├── token.go          # Bearer token (cached)
│   ├── forecast.go       # STUB — returns mock data; GRIB2 decoding not yet implemented
│   └── avalanche.go      # Real DPBRA API call; falls back to mock on error
├── schedule/
│   └── naismith.go       # duration = distance_km/5 + elevation_m/600
└── pdf/
    └── export.go         # chromedp headless Chromium → PDF bytes
```

### Step 2.3 — CampToCamp integration ✅
- **No mock fallbacks in production code.** Errors and empty results are returned as-is.
- Search supports two modes via `location_type`:
  - `"name"` → C2C `q=<text>` parameter
  - `"location"` → geocode via Nominatim (parses `"lat,lon"` directly if given), then C2C `bbox=<minx,miny,maxx,maxy>` in EPSG:3857 with 20 km radius
- Route detail extracts `geometry.geom` (stringified GeoJSON in EPSG:3857) and converts to WGS84 (`lat`/`lon`) for the map
- `alternative_routes` always serialises as `[]` (never JSON `null`) to avoid frontend crashes
- Test infrastructure: `baseURL` and `nominatimBaseURL` are package-level `var`s, overridden in unit tests via `httptest.Server`

### Step 2.4 — MeteoFrance integration 🔄
- **Token** (`meteo/token.go`): real implementation, caches Bearer token in memory
- **Weather forecast** (`meteo/forecast.go`): **STUB** — returns mock data. GRIB2 decoding with `eccodes-go` not yet implemented. The Dockerfile installs `libeccodes-dev`. TODO: implement using `github.com/meteocima/eccodes-go` CGO bindings.
- **Avalanche bulletin** (`meteo/avalanche.go`): calls real DPBRA API; falls back to mock on failure

### Step 2.5 — Schedule computation ✅
`schedule/naismith.go`: `duration_hours = (distance_km / 5.0) + (elevation_gain_m / 600.0)`, minimum 4 h.
`source` = `"camptocamp"` when `time_required` is present in C2C locales, else `"formula"`.

### Step 2.6 — PDF export ✅
`pdf/export.go` uses `chromedp` to render the frontend page and print to landscape A4 PDF.
Works in Docker (Chromium installed); not available in the devcontainer.

### Step 2.7 — Gin wiring ✅
`main.go` loads `.env`, registers all `/api/*` routes, serves `./static` for everything else.

### Step 2.8 — Backend unit tests ✅
- `camptocamp/search_test.go` — 6 tests: name search, empty results, API error, raw GPS bypass, Nominatim geocoding, Nominatim failure
- `camptocamp/detail_test.go` — 8 tests: title/description/difficulty, geometry → WGS84, `alternative_routes` never null, schedule source (Naismith vs C2C), API error, mock completeness guard
- `schedule/naismith_test.go` — formula correctness
- Mock data (`mockDetail`, `mockSearchResults`) lives **only in test files**, not in production code
- Run with: `go test ./...` from `backend/`

---

## Phase 3 — Frontend ✅

### Step 3.1 — Next.js initialisation ✅
Next.js 16 with TypeScript, Tailwind, App Router, `output: 'export'`, `trailingSlash: true`.
Dependencies: `next-intl`, `leaflet`, `react-leaflet`, `recharts`.

### Step 3.2 — i18n ✅
`next-intl` configured with `messages/fr.json` and `messages/en.json`.
Locale auto-detected from `navigator.language`. All visible strings translated.

### Step 3.3 — Design system ✅
Primary `#1F2782`, white surfaces, mountain-theme Tailwind config.
Panel cards with consistent `panel` / `panel-header` / `panel-body` class pattern.

### Step 3.4 — Page layout ✅
CSS Grid, 3 columns (`210px 1fr 270px`), 4 rows. Grid areas: `p1`, `top-mid`, `p3`, `p5`, `p6`, `p9`, `bot-mid`.

### Step 3.5 — Part 1: Participants ✅
Dynamic add/remove. Name input + climbing level select (French grades).

### Step 3.6 — Part 2: Objectives ✅
Checkbox group (Challenge / Fun / Performance / Discovery) + free text notes.

### Step 3.7 — Part 4: Race search ✅
- Date, race type (drives difficulty scale), difficulty, location with **name / location toggle**
- Location toggle: `"name"` sends `q` text search; `"location"` geocodes and sends bbox search
- On route selection: search results list is cleared immediately
- Calls `POST /api/routes/search`, displays result list, on click fetches `GET /api/routes/:id` + `GET /api/weather` in parallel

### Step 3.8 — Part 3: Weather ✅
Temperature range, precipitation, wind, avalanche risk badge (colour-coded 1–5).

### Step 3.9 — Part 5: Race detail ✅
Three tabs: **Topo/Description** (pitch table or description text), **Map** (Leaflet, centred on real route GPS from C2C geometry), **Elevation profile** (synthetic Naismith-based profile via Recharts — real GPX not decoded yet).
Map uses `route.lat` / `route.lon` from the API response; defaults to Chamonix (45.9, 6.9) if coordinates are missing.

### Step 3.10 — Part 6: Risks ✅
Bulleted list from C2C `remarks` and `risk` locale fields.

### Step 3.11 — Part 7: Alternatives ✅
List from `associations.routes`. Each item links to CampToCamp URL.

### Step 3.12 — Part 8: Schedule ✅
Estimated duration, start/end times. Naismith notice banner shown when `source === "formula"`.

### Step 3.13 — Part 9: Equipment ✅
Table from C2C `gear` / `equipment_rating` locale fields.

### Step 3.14 — PDF export ✅
Header button POSTs to `/api/export/pdf`, triggers browser file download.

### Step 3.15 — Frontend unit tests ⬜
Not yet implemented. React Testing Library tests for each panel component are planned.

---

## Phase 4 — Integration & Docker build ✅

### Step 4.1 — Full build ✅
`make build` and `make run` validated. `make local-build` + `make local-run` also available for development without Docker.

### Step 4.2 — API smoke tests ✅
All four endpoints return correct status codes and shapes. C2C integration confirmed live against `api.camptocamp.org`.

---

## Phase 5 — E2E Tests ⬜

Not started. Infrastructure and test scenarios remain to be implemented:

### Step 5.1 — test/docker-compose.test.yml
Two services:
- `app`: production image from `Dockerfile`
- `playwright`: `mcr.microsoft.com/playwright` running the test suite

### Step 5.2 — Playwright test scenarios

| Scenario | Status |
|---|---|
| Page load — all 9 panels visible | ⬜ |
| Add participant | ⬜ |
| Race type change → difficulty scale changes | ⬜ |
| Search by name → results list renders | ⬜ |
| Search by location → geocode + bbox used | ⬜ |
| Route selection → panels 3/5/6/7/8/9 fill in | ⬜ |
| Schedule formula notice shown | ⬜ |
| Weather error → graceful state | ⬜ |
| PDF export → download triggered | ⬜ |

---

## Phase 6 — Final checks 🔄

| Check | Status |
|---|---|
| All frontend text has FR/EN translations | ✅ |
| `#1F2782` primary colour applied consistently | ✅ |
| `.env.example` committed, `.env` gitignored | ✅ |
| No mock fallbacks in production backend code | ✅ |
| `alternative_routes` never serialises as null | ✅ |
| Map uses real GPS coordinates from C2C | ✅ |
| Weather uses route GPS (not hardcoded) | ⬜ TODO: page.tsx still passes hardcoded `lat=45.9&lon=6.9` to `/api/weather`; should use route lat/lon |
| Elevation profile uses real GPX data | ⬜ TODO: currently synthetic (Naismith-shaped curve); needs GPX decoding |
| GRIB2 weather decoding | ⬜ TODO: `meteo/forecast.go` returns mock; implement with `eccodes-go` |
| Frontend unit tests (React Testing Library) | ⬜ |
| E2E tests (Playwright) | ⬜ |
| Responsive tablet layout | 🔄 Desktop-first implemented; tablet stacking not tested |

---

## Known Limitations & TODOs

1. **GRIB2 forecast** (`backend/meteo/forecast.go`): stub returning mock data. Real implementation needs `github.com/meteocima/eccodes-go` CGO bindings + `libeccodes-dev` already installed in the Docker image.

2. **Weather API call uses hardcoded coordinates**: `page.tsx` calls `/api/weather?lat=45.9&lon=6.9`. Should pass the selected route's `lat`/`lon` instead.

3. **Elevation profile is synthetic**: `DetailPanel.tsx` generates a bell-curve profile from elevation gain and distance. Real GPX track decoding is not yet implemented.

4. **Nominatim rate limiting**: production use should cache geocoding results or use a self-hosted instance to avoid Nominatim's 1 req/s limit.
