# Mountain Race

This entire project is to be built and tested according to this specification.
It must be tested by running within a Docker container.

## Project Specification

## 1. Vision

The objective is to help user on setting up a mountain race with his friends. This race can either be:
- a climbing multi pitch route
- a ridge hike
- a basic hike

He can fill in parameters to give input to the backend, which returns a list of routes that match the criteria. 
Then the user selects a route, and all the informations related to this route are displayed, along with external parameters like the weather conditions, avalanche, ...

All the application front end texts should be localized in french and english, default language being the browser language.

## 2. User Experience

### First Launch
When launching the application, it should display a single page as described in the **Displaying the course scheduling** section. The page is pre-filled with empty state and all inputs are editable.

The inputs required are:
- The number of persons
- For each person: 
    - their name
    - their climbing level. this level should be based on the french climbing grade
- The race informations
    - The date where the race is scheduled
    - The race type:
        - a climbing multi pitch route
        - a ridge hike
        - a basic hike
    - The race difficulty. The difficulty scale adapts automatically based on the race type:
        - **Multipitch climbing**: French sport grade scale (4a → 9c)
        - **Ridge hike or basic hike**: Alpine cotation scale (F, PD, AD, D, TD, ED)
    - A destination or GPS position to help on finding a race
    - A Search button to launch the race search

Finding a race after launching the search is explained into the **Finding a race** section.
Once a race is selected, the relevant parts of the page are filled in as explained in the **Displaying the course scheduling** section.

### What the User Can Do

- **Refine the race information** — He can edit all the input parameters described above at any time
- **Export the race result** - The informations displayed like explained in the **Displaying the course scheduling** should be exported in a visually stunning PDF file (landscape, rendered via headless Chromium)

### Finding a race ###
In order to find a race that matches the user inputs, the backend should look for information from the following source:
- CampToCamp website (`https://www.camptocamp.org`): use the user's desired location, the group's climbing grade, and the race type via the CampToCamp API. As there is no official API, rely on the github repo `https://github.com/c2corg/v6_api` which contains the API code. Access is anonymous read-only (no credentials required). All climbing grades returned must be in the French grading format.

### Getting weather informations ###
In order to get weather information related to the race, the backend should use the following MeteoFrance APIs. Both require a Bearer token obtained by POSTing HTTP Basic Auth (METEOFRANCE_USER / METEOFRANCE_PASS) to `https://portail-api.meteofrance.fr/token`.

- **Point weather forecast**: Use the MeteoFrance **AROME** API (`https://public-api.meteofrance.fr/public/arome/1.0`) if race is scheduled within 48 hours, or **ARPEGE** (`https://public-api.meteofrance.fr/public/arpege/1.0`) if not. Use it to retrieve the grib2 file content, and get the forecast data (temperature, rain, isotherm, ...)

- **Avalanche forecast**: Use the MeteoFrance **DPBRA** API (`https://public-api.meteofrance.fr/public/DPBRA/v1`) to retrieve the Bulletin de Risque d'Avalanche for the relevant massif.

### Visual Design

- **Mountain theme**: the theme should be inspired by the mountains, climbing, snow
- **Responsive but desktop-first**: optimized for wide screens, functional on tablet

### Color Scheme
- The main colors are `#1F2782` (dark blue) and `#FFFFFF` (white), evoking the mountains

## 3. Architecture Overview

### Development environment
Nothing should be installed directly on the local machine. We're using a devcontainer to encapsulate all the needed libraries, frameworks, etc.
The `Dockerfile.devcontainer` located at the root of the project is used for the devcontainer purpose and should be updated if something is missing in the development environment.

### Single Container, Single Port

```
┌─────────────────────────────────────────────────┐
│  Docker Container (port 8003)                   │
│                                                 │
│  Gin (Golang)                                   │
│  ├── /api/*          REST endpoints             │
│  └── /*              Static file serving        │
│                      (Next.js export)           │
│                                                 │
└─────────────────────────────────────────────────┘
```

- **Frontend**: Next.js with TypeScript, built as a static export (`output: 'export'`), served by Gin as static files
- **Backend**: Gin framework (Go)


## 4. Directory Structure

```
mountain-race/
├── frontend/                 # Next.js TypeScript project (static export)
├── backend/                  # Gin project (Go)
├── planning/                 # Project-wide documentation for agents
│   └── ...                   # Additional agent reference docs
├── Makefile                  # build (build the Docker), run (launch the Docker)
├── test/                     # Playwright E2E tests + docker-compose.test.yml
├── Dockerfile                # Multi-stage production build (Go → Node → final image)
├── Dockerfile.devcontainer   # Devcontainer image for VS Code. Nothing is installed on the local machine.
├── docker-compose.yml        # Optional convenience wrapper
├── .env                      # Environment variables (gitignored, .env.example committed)
└── .gitignore
```

### Key Boundaries

- **`frontend/`** is a self-contained Next.js project. It knows nothing about Go. It talks to the backend via `/api/*` endpoints.
- **`backend/`** is a self-contained Go project. It owns all server logic including API routes.
- **`planning/`** contains project-wide documentation. All agents reference files here as the shared contract.
- **`test/`** contains Playwright E2E tests and supporting infrastructure (e.g., `docker-compose.test.yml`). Unit tests live within `frontend/` and `backend/` respectively.

---

## 5. Environment Variables
- **METEOFRANCE_USER**: MeteoFrance API username, used to generate a Bearer token
- **METEOFRANCE_PASS**: MeteoFrance API password, used to generate a Bearer token

### Behavior

- The backend reads `.env` from the project root (mounted into the container or read via docker `--env-file`)

---

## 6. API Endpoints

All endpoints are prefixed with `/api`.

### Routes

#### `POST /api/routes/search`
Search for routes matching user criteria via CampToCamp.

**Request body:**
```json
{
  "location": "string",          // route name, place name, or "lat,lon"
  "location_type": "name|location", // "name" = text search via C2C q param; "location" = geocode then bbox search
  "race_type": "multipitch|ridge_hike|hike",
  "difficulty": "string",        // French sport grade (e.g. "5c") for multipitch; alpine cotation (e.g. "AD") for hikes
  "date": "2006-01-02",
  "participants": [
    { "name": "string", "climbing_level": "string" }
  ]
}
```

**Response `200`:**
```json
{
  "routes": [
    {
      "id": "string",
      "title": "string",
      "summary": "string",
      "difficulty": "string",
      "elevation_gain": 0,
      "distance_km": 0.0,
      "source_url": "string"
    }
  ]
}
```

---

#### `GET /api/routes/:id`
Fetch full detail for a single CampToCamp route.

**Response `200`:**
```json
{
  "id": "string",
  "title": "string",
  "description": "string",
  "difficulty": "string",
  "elevation_gain": 0,
  "distance_km": 0.0,
  "lat": 0.0,                    // WGS84 latitude, extracted from C2C geometry (EPSG:3857 → WGS84)
  "lon": 0.0,                    // WGS84 longitude
  "pitches": [                   // only for multipitch
    { "number": 1, "grade": "string", "description": "string" }
  ],
  "topo_url": "string",          // link to topo image if available
  "gpx_url": "string",           // link to GPX track if available
  "equipment": [
    { "item": "string", "quantity": 0, "notes": "string" }
  ],
  "risks": ["string"],
  "alternative_routes": [
    { "id": "string", "title": "string", "reason": "string" }
  ],
  "schedule": {
    "estimated_duration_hours": 0.0,
    "recommended_start_time": "06:00",
    "recommended_end_time": "16:00",
    "source": "camptocamp|formula"
    // "formula" = Naismith's rule applied to distance + elevation; UI must show a clear notice to the user
  },
  "source_url": "string"
}
```

---

#### `GET /api/weather`
Fetch weather forecast and avalanche risk for a location and date.

**Query params:** `lat`, `lon`, `date` (YYYY-MM-DD)

**Response `200`:**
```json
{
  "forecast": {
    "date": "2006-01-02",
    "temperature_min_c": 0.0,
    "temperature_max_c": 0.0,
    "precipitation_mm": 0.0,
    "wind_speed_kmh": 0.0,
    "condition": "string"         // e.g. "sunny", "snow", "rain"
  },
  "avalanche": {
    "risk_level": 0,              // 1–5 European scale
    "risk_label": "string",       // e.g. "Limité"
    "description": "string"
  }
}
```

---

#### `POST /api/export/pdf`
Generate and return a PDF of the full race plan using headless Chromium. Output is landscape A4.

**Request body:** same shape as `GET /api/routes/:id` response, plus weather block.

**Response `200`:** `application/pdf` binary stream.

---

## 7. Displaying the course scheduling

All the user experience is displayed on a single page divided into 9 parts. Part 5 takes the most space and displays the selected route.

Parts:
- **Part 1: Participants.** User adds all people participating with their information (name, climbing level).
- **Part 2: Objectives.** User enters group objectives: challenge, fun, performance, etc.
- **Part 3: Weather conditions.** Displays weather forecast (rain, snow, wind, temperature) and avalanche risk level for the race date. Sourced from MeteoFrance APIs. Filled after a race is selected.
- **Part 4: Race search.** Inputs as described in **First Launch** (date, type, difficulty, location, participants). A Search button launches the search and displays a list of matching routes. Selecting a route fills Parts 3, 5, 6, 7, 8, and 9.
- **Part 5: Race detail.** Filled when a race is selected. Displays: route topo (pitch-by-pitch with French grades for multipitch), elevation profile, map view with GPX track, and full route description.
- **Part 6: Risks, points of vigilance.** Filled when a race is selected. Data from CampToCamp user comments and the route's global description.
- **Part 7: Alternative routes.** Filled when a race is selected. Other routes in case of difficulty: easier fallback, return point, etc.
- **Part 8: Schedule.** Filled when a race is selected. Estimated duration and recommended start/end times. Sourced from CampToCamp user comments when available; otherwise computed via Naismith's rule — a clear notice is shown to the user in this case.
- **Part 9: Equipment.** Filled when a race is selected. Equipment list sourced from CampToCamp (e.g. number of quickdraws, rope length, crampons).

Layout:

```
┌─────────────────────────────────────────────────┐
│ Part 1  │ Part 2 | Part 4          | Part 3     |
│         |_________________________ |____________│
│         |                          │ Part 6     |
│_________| Part 5                   │            |
│ Part 9  │                          |            |
│         │__________________________|            |
│         |Part 8       |     Part 7 |            │
└─────────────────────────────────────────────────┘
```

### Technical Notes

- **GRIB2 decoding**: AROME and ARPEGE return forecast data as GRIB2 binary files. The Go backend decodes them using `github.com/meteocima/eccodes-go` (CGO bindings to the eccodes C library). The `eccodes` C library must be installed in the Docker image. No subprocess shelling-out.
- **Model selection**: use AROME (2.5 km resolution, max +48 h) for race dates within 2 days of today; use ARPEGE (global model, up to +4 days) for dates beyond that.

---

## 8. Docker & Deployment

### Multi-Stage Dockerfile

```
Stage 1: golang:1.26.2
  - apt-get install libeccodes-dev
  - Compile Go backend binary (CGO_ENABLED=1)

Stage 2: node:24-slim
  - Install Chromium (for PDF export via headless browser)
  - Install eccodes (ECMWF library for GRIB2 decoding of AROME/ARPEGE forecast files)
  - Copy frontend/
  - npm install && npm run build (produces static export into out/)
  - Copy Go binary from Stage 1
  - Copy frontend static output into static/
  - Expose port 8003
  - Launch Go binary
```

Gin serves static frontend files and all `/api/*` routes on port 8003.

### Makefile targets

| Target | Description |
|---|---|
| `make build` | Build the Docker image (`mountain-race`) |
| `make run` | Run the Docker container on port 8003, loading `.env` |
| `make local-build` | Build frontend → copy static files → build Go binary, all without Docker |
| `make local-run` | Run the Go binary locally (after `local-build`); serves on port 8003 |

### Optional Cloud Deployment

The container is designed to deploy to AWS App Runner, Render, or any container platform. A Terraform configuration for App Runner may be provided in a `deploy/` directory as a stretch goal, but is not part of the core build.

---

## 9. Testing Strategy

### Unit Tests (within `frontend/` and `backend/`)

**Backend**:
- API routes: correct status codes, response shapes, error handling

**Frontend (React Testing Library)**:
- Component rendering with mock data

### E2E Tests (in `test/`)

**Infrastructure**: A separate `docker-compose.test.yml` in `test/` spins up the app container plus a Playwright container. This keeps browser dependencies out of the production image.

**Key Scenarios**:
- Test all external API request scenarios (success, failure) in the backend
