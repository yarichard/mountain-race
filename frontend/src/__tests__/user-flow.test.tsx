/**
 * Integration test: typical user flow
 * Renders the full Home page with real i18n, mocked fetch.
 * No running server required.
 */
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import Home from "@/app/page";

// --- Module mocks ---

// Resolve real English strings without importing the ESM next-intl bundle
// eslint-disable-next-line @typescript-eslint/no-require-imports
const en = require("../../messages/en.json");

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getTranslation(ns: string | undefined, key: string): string {
  const parts = ns ? [ns, ...key.split(".")] : key.split(".");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let node: any = en;
  for (const part of parts) {
    node = node?.[part];
  }
  return typeof node === "string" ? node : `${ns ?? ""}.${key}`;
}

jest.mock("next-intl", () => ({
  useTranslations: (ns?: string) => (key: string) => getTranslation(ns, key),
  NextIntlClientProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

jest.mock("@/components/MapView", () => () => <div data-testid="map-view" />);

jest.mock("recharts", () => {
  const actual = jest.requireActual("recharts");
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
  };
});

// --- Fixtures ---

const MOCK_ROUTE_RESULT = {
  id: "789",
  title: "Arête des Cosmiques",
  summary: "Classique du Mont-Blanc",
  difficulty: "AD",
  elevation_gain: 300,
  distance_km: 2.5,
  source_url: "https://www.camptocamp.org/routes/789",
};

const MOCK_ROUTE_DETAIL = {
  id: "789",
  title: "Arête des Cosmiques",
  description: "Belle arête mixte au-dessus de Chamonix.",
  difficulty: "AD",
  elevation_gain: 300,
  distance_km: 2.5,
  lat: 45.87,
  lon: 6.88,
  pitches: [],
  topo_url: "",
  gpx_url: "",
  equipment: [{ item: "Crampons", quantity: 1, notes: "" }],
  risks: ["Risque de glace"],
  alternative_routes: [],
  schedule: {
    estimated_duration_hours: 3,
    recommended_start_time: "06:00",
    recommended_end_time: "12:00",
    source: "camptocamp",
  },
  source_url: "https://www.camptocamp.org/routes/789",
};

const MOCK_WEATHER = {
  forecast: {
    date: "2026-04-20",
    temperature_min_c: -2,
    temperature_max_c: 8,
    precipitation_mm: 0,
    wind_speed_kmh: 20,
    condition: "sunny",
  },
  avalanche: {
    risk_level: 2,
    risk_label: "Limité",
    description: "Risque limité en altitude.",
  },
  hourly: [
    { hour: 0, temperature_c: -2, wind_speed_kmh: 15 },
    { hour: 3, temperature_c: -3, wind_speed_kmh: 18 },
  ],
};

// --- Helpers ---

function renderApp() {
  return render(<Home />);
}

type FetchOverrides = {
  searchRoutes?: unknown[];
  routeDetail?: unknown;
  weather?: unknown;
};

function mockFetch(overrides: FetchOverrides = {}) {
  const spy = jest.fn().mockImplementation(async (input: RequestInfo) => {
    const url = typeof input === "string" ? input : (input as Request).url;
    if (url === "/api/routes/search") {
      const routes = overrides.searchRoutes ?? [MOCK_ROUTE_RESULT];
      return { ok: true, json: async () => ({ routes }) } as Response;
    }
    if (url.startsWith("/api/routes/")) {
      return { ok: true, json: async () => overrides.routeDetail ?? MOCK_ROUTE_DETAIL } as Response;
    }
    if (url.startsWith("/api/weather")) {
      return { ok: true, json: async () => overrides.weather ?? MOCK_WEATHER } as Response;
    }
    return { ok: false, status: 404, json: async () => ({}) } as Response;
  });
  global.fetch = spy;
  return spy;
}

// --- Tests ---

describe("User flow: search and select a route", () => {
  afterEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (global as any).fetch;
    jest.restoreAllMocks();
  });

  it("renders all key panels in initial empty state", () => {
    renderApp();
    expect(screen.getByText("Participants")).toBeInTheDocument();
    expect(screen.getAllByText("Search").length).toBeGreaterThan(0);
    expect(screen.getByText("Weather")).toBeInTheDocument();
    expect(screen.getByText("Route detail")).toBeInTheDocument();
    expect(screen.getByText("Select a route to see details")).toBeInTheDocument();
    expect(screen.getByText("Select a route to see weather")).toBeInTheDocument();
  });

  it("fills in the location and triggers a search, results appear", async () => {
    const fetchSpy = mockFetch();
    renderApp();

    fireEvent.change(screen.getByPlaceholderText(/Aiguille du Midi/i), {
      target: { value: "Chamonix" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));

    expect(fetchSpy).toHaveBeenCalledWith(
      "/api/routes/search",
      expect.objectContaining({ method: "POST" })
    );

    await waitFor(() =>
      expect(screen.getByText("Arête des Cosmiques")).toBeInTheDocument()
    );
    expect(screen.getByText("↑300m · 2.5km")).toBeInTheDocument();
  });

  it("shows loading spinner in detail panel while fetching route", async () => {
    // Hold the route detail fetch until we explicitly release it
    let releaseRoute!: (v: unknown) => void;
    const holdRoute = new Promise((res) => { releaseRoute = res; });

    global.fetch = jest.fn().mockImplementation(async (input: RequestInfo) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url === "/api/routes/search") {
        return { ok: true, json: async () => ({ routes: [MOCK_ROUTE_RESULT] }) } as Response;
      }
      if (url.startsWith("/api/routes/")) {
        await holdRoute;
        return { ok: true, json: async () => MOCK_ROUTE_DETAIL } as Response;
      }
      return { ok: false, json: async () => ({}) } as Response;
    });

    renderApp();

    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));
    await waitFor(() => expect(screen.getByText("Arête des Cosmiques")).toBeInTheDocument());

    fireEvent.click(screen.getByText("Arête des Cosmiques"));

    await waitFor(() =>
      expect(screen.getByText("Loading route from CampToCamp...")).toBeInTheDocument()
    );

    // Release the fetch and verify spinner disappears
    act(() => releaseRoute(undefined));
    await waitFor(() =>
      expect(screen.queryByText("Loading route from CampToCamp...")).not.toBeInTheDocument()
    );
  });

  it("shows loading spinner in weather panel while fetching weather", async () => {
    let releaseWeather!: (v: unknown) => void;
    const holdWeather = new Promise((res) => { releaseWeather = res; });

    global.fetch = jest.fn().mockImplementation(async (input: RequestInfo) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url === "/api/routes/search") {
        return { ok: true, json: async () => ({ routes: [MOCK_ROUTE_RESULT] }) } as Response;
      }
      if (url.startsWith("/api/routes/")) {
        return { ok: true, json: async () => MOCK_ROUTE_DETAIL } as Response;
      }
      if (url.startsWith("/api/weather")) {
        await holdWeather;
        return { ok: true, json: async () => MOCK_WEATHER } as Response;
      }
      return { ok: false, json: async () => ({}) } as Response;
    });

    renderApp();

    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));
    await waitFor(() => expect(screen.getByText("Arête des Cosmiques")).toBeInTheDocument());

    fireEvent.click(screen.getByText("Arête des Cosmiques"));

    // Route loads, then weather spinner kicks in
    await waitFor(() =>
      expect(screen.getByText("Fetching weather forecast...")).toBeInTheDocument()
    );

    act(() => releaseWeather(undefined));
    await waitFor(() =>
      expect(screen.queryByText("Fetching weather forecast...")).not.toBeInTheDocument()
    );
  });

  it("full flow: select a route, detail and weather panels are populated", async () => {
    mockFetch();
    renderApp();

    // Search
    fireEvent.change(screen.getByPlaceholderText(/Aiguille du Midi/i), {
      target: { value: "Chamonix" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));
    await waitFor(() => expect(screen.getByText("Arête des Cosmiques")).toBeInTheDocument());

    // Select the result
    fireEvent.click(screen.getByText("Arête des Cosmiques"));

    // Detail panel: description appears once detail loads
    await waitFor(() =>
      expect(screen.getByText("Belle arête mixte au-dessus de Chamonix.")).toBeInTheDocument()
    );

    // Weather panel: data is shown
    await waitFor(() => expect(screen.getByText("-2°C")).toBeInTheDocument());
    expect(screen.getByText("8°C")).toBeInTheDocument();
    expect(screen.getByText(/Limité/)).toBeInTheDocument();
  });

  it("changing the date while a route is selected immediately re-fetches weather", async () => {
    const fetchSpy = mockFetch();
    renderApp();

    // Search and select a route
    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));
    await waitFor(() => expect(screen.getByText("Arête des Cosmiques")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Arête des Cosmiques"));
    await waitFor(() => expect(screen.getByText("Belle arête mixte au-dessus de Chamonix.")).toBeInTheDocument());

    const weatherCallsAfterSelection = fetchSpy.mock.calls.filter(
      ([url]: [string]) => typeof url === "string" && url.startsWith("/api/weather")
    ).length;
    expect(weatherCallsAfterSelection).toBe(1);

    // Change the date → weather should re-fetch immediately (route already selected)
    const dateInput = screen.getByDisplayValue(/^\d{4}-\d{2}-\d{2}$/);
    fireEvent.change(dateInput, { target: { value: "2026-06-15" } });

    await waitFor(() => {
      const weatherCalls = fetchSpy.mock.calls.filter(
        ([url]: [string]) => typeof url === "string" && url.startsWith("/api/weather")
      ).length;
      expect(weatherCalls).toBe(2);
    });

    // Verify the new date is used in the weather request
    const weatherUrls = fetchSpy.mock.calls
      .filter(([url]: [string]) => typeof url === "string" && url.startsWith("/api/weather"))
      .map(([url]: [string]) => url as string);
    expect(weatherUrls[1]).toContain("date=2026-06-15");
  });

  it("changing the date without a selected route does not fetch weather", async () => {
    const fetchSpy = mockFetch();
    renderApp();

    // Change the date before any route is selected
    const dateInput = screen.getByDisplayValue(/^\d{4}-\d{2}-\d{2}$/);
    fireEvent.change(dateInput, { target: { value: "2026-06-15" } });

    // Give any potential async side-effects time to fire
    await new Promise((r) => setTimeout(r, 50));

    const weatherCalls = fetchSpy.mock.calls.filter(
      ([url]: [string]) => typeof url === "string" && url.startsWith("/api/weather")
    ).length;
    expect(weatherCalls).toBe(0);
  });

  it("shows no results message when search returns empty list", async () => {
    mockFetch({ searchRoutes: [] });
    renderApp();

    fireEvent.click(screen.getByRole("button", { name: /^Search$/ }));

    await waitFor(() =>
      expect(screen.getByText("No routes found")).toBeInTheDocument()
    );
  });
});
