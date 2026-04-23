import { render, screen } from "@testing-library/react";
import { DetailPanel } from "../DetailPanel";
import type { RouteDetail } from "@/lib/types";

jest.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string) => `${ns}.${key}`,
}));

// MapView uses dynamic() with ssr:false — stub it out
jest.mock("../MapView", () => () => <div data-testid="map-view" />);

// Recharts ResizeObserver isn't available in jsdom
jest.mock("recharts", () => {
  const actual = jest.requireActual("recharts");
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
  };
});

const mockRoute: RouteDetail = {
  id: "123",
  title: "Voie des Cristaux",
  description: "Belle voie sur granit.",
  difficulty: "5c",
  elevation_gain: 400,
  height_diff_down: 400,
  lat: 45.9,
  lon: 6.9,
  gear_text: "",
  gpx_url: "",
  equipment: [],
  risks: [],
  alternative_routes: [],
  schedule: {
    estimated_duration_hours: 4,
    recommended_start_time: "07:00",
    recommended_end_time: "14:00",
    source: "camptocamp",
  },
  source_url: "https://www.camptocamp.org/routes/123",
};

describe("DetailPanel", () => {
  it("shows empty state when no route and not loading", () => {
    render(<DetailPanel route={null} />);
    expect(screen.getByText("detail.empty")).toBeInTheDocument();
  });

  it("shows loading spinner when loading=true", () => {
    render(<DetailPanel route={null} loading />);
    expect(screen.getByText("detail.loading")).toBeInTheDocument();
    expect(document.querySelector("svg")).toBeInTheDocument();
  });

  it("does not show loading text when loading=false", () => {
    render(<DetailPanel route={null} loading={false} />);
    expect(screen.queryByText("detail.loading")).not.toBeInTheDocument();
  });

  it("renders route title and difficulty when route is provided", () => {
    render(<DetailPanel route={mockRoute} />);
    expect(screen.getByText("Voie des Cristaux")).toBeInTheDocument();
    expect(screen.getAllByText("5c").length).toBeGreaterThan(0);
  });

  it("renders route description", () => {
    render(<DetailPanel route={mockRoute} />);
    expect(screen.getByText("Belle voie sur granit.")).toBeInTheDocument();
  });
});
