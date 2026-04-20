import { render, screen } from "@testing-library/react";
import { WeatherPanel } from "../WeatherPanel";
import type { WeatherData } from "@/lib/types";

jest.mock("next-intl", () => ({
  useTranslations: (ns: string) => (key: string) => `${ns}.${key}`,
}));

jest.mock("recharts", () => {
  const actual = jest.requireActual("recharts");
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  };
});

const MOCK_HOURLY = [
  { hour: 0, temperature_c: 5, wind_speed_kmh: 10 },
  { hour: 3, temperature_c: 4, wind_speed_kmh: 12 },
  { hour: 6, temperature_c: 6, wind_speed_kmh: 8 },
];

const mockWeather: WeatherData = {
  forecast: {
    date: "2026-04-20",
    temperature_min_c: 2,
    temperature_max_c: 12,
    precipitation_mm: 0,
    wind_speed_kmh: 15,
    condition: "sunny",
  },
  avalanche: {
    risk_level: 2,
    risk_label: "Limité",
    description: "Risque limité en altitude.",
  },
};

const mockWeatherWithHourly: WeatherData = { ...mockWeather, hourly: MOCK_HOURLY };

describe("WeatherPanel", () => {
  it("shows empty state when no weather and not loading", () => {
    render(<WeatherPanel weather={null} />);
    expect(screen.getByText("weather.empty")).toBeInTheDocument();
  });

  it("shows loading spinner when loading=true", () => {
    render(<WeatherPanel weather={null} loading />);
    expect(screen.getByText("weather.loading")).toBeInTheDocument();
    expect(document.querySelector("svg")).toBeInTheDocument();
  });

  it("does not show loading text when loading=false", () => {
    render(<WeatherPanel weather={null} loading={false} />);
    expect(screen.queryByText("weather.loading")).not.toBeInTheDocument();
  });

  it("shows error state when error=true and not loading", () => {
    render(<WeatherPanel weather={null} error />);
    expect(screen.getByText("weather.error")).toBeInTheDocument();
  });

  it("shows loading spinner over error when both loading and error", () => {
    render(<WeatherPanel weather={null} loading error />);
    expect(screen.getByText("weather.loading")).toBeInTheDocument();
    expect(screen.queryByText("weather.error")).not.toBeInTheDocument();
  });

  it("renders weather data when weather is provided", () => {
    render(<WeatherPanel weather={mockWeather} />);
    expect(screen.getByText("2°C")).toBeInTheDocument();
    expect(screen.getByText("12°C")).toBeInTheDocument();
    expect(screen.getByText("Limité (2/5)")).toBeInTheDocument();
  });

  it("does not render chart when no hourly data", () => {
    render(<WeatherPanel weather={mockWeather} />);
    expect(screen.queryByTestId("hourly-chart")).not.toBeInTheDocument();
  });

  it("renders chart when hourly data is present", () => {
    render(<WeatherPanel weather={mockWeatherWithHourly} />);
    expect(screen.getByTestId("hourly-chart")).toBeInTheDocument();
  });
});
