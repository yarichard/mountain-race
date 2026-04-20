"use client";

import { useTranslations } from "next-intl";
import type { WeatherData } from "@/lib/types";
import { LoadingSpinner } from "./LoadingSpinner";
import {
  ComposedChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from "recharts";

interface Props {
  weather: WeatherData | null;
  loading?: boolean;
  error?: boolean;
}

const RISK_COLORS = ["", "text-green-600", "text-lime-600", "text-orange-500", "text-red-600", "text-black font-bold"];
const RISK_BG = ["", "bg-green-100", "bg-lime-100", "bg-orange-100", "bg-red-100", "bg-gray-900 text-white"];

const CONDITION_ICONS: Record<string, string> = {
  sunny: "☀️",
  partly_cloudy: "⛅",
  cloudy: "☁️",
  rain: "🌧️",
  snow: "❄️",
  storm: "⛈️",
};

const CHART_TEMP_COLOR = "#1F2782";
const CHART_WIND_COLOR = "#e57c2b";

export function WeatherPanel({ weather, loading, error }: Props) {
  const t = useTranslations("weather");

  if (!weather) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className={`panel-body flex-1 flex items-center justify-center text-sm text-center ${error ? "text-red-600" : "text-[var(--text-muted)]"}`}>
          {loading ? (
            <LoadingSpinner message={t("loading")} />
          ) : error ? (
            t("error")
          ) : (
            t("empty")
          )}
        </div>
      </div>
    );
  }

  const { forecast, avalanche } = weather;
  const icon = CONDITION_ICONS[forecast.condition] ?? "🏔️";
  const conditionLabel = t(`condition.${forecast.condition}` as Parameters<typeof t>[0]) ?? forecast.condition;

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 space-y-3">
        <div className="flex items-center gap-3">
          <span className="text-4xl">{icon}</span>
          <div>
            <p className="font-semibold text-[var(--primary)]">{conditionLabel}</p>
            <p className="text-sm text-[var(--text-muted)]">{forecast.date}</p>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
          <span className="text-[var(--text-muted)]">{t("tempMin")}</span>
          <span className="font-medium">{forecast.temperature_min_c.toFixed(0)}°C</span>
          <span className="text-[var(--text-muted)]">{t("tempMax")}</span>
          <span className="font-medium">{forecast.temperature_max_c.toFixed(0)}°C</span>
          <span className="text-[var(--text-muted)]">{t("precipitation")}</span>
          <span className="font-medium">{forecast.precipitation_mm.toFixed(1)} mm</span>
          <span className="text-[var(--text-muted)]">{t("wind")}</span>
          <span className="font-medium">{forecast.wind_speed_kmh.toFixed(0)} km/h</span>
        </div>
        {weather.hourly && weather.hourly.length > 0 && (
          <div data-testid="hourly-chart">
            <ResponsiveContainer width="100%" height={110}>
              <ComposedChart data={weather.hourly} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
                <XAxis
                  dataKey="hour"
                  tick={{ fontSize: 10 }}
                  tickFormatter={(h: number) => `${h}h`}
                />
                <YAxis yAxisId="temp" tick={{ fontSize: 10 }} />
                <YAxis yAxisId="wind" orientation="right" tick={{ fontSize: 10 }} />
                <Tooltip
                  formatter={(val, name) =>
                    name === t("chart.temp") ? [`${val}°C`, name] : [`${val} km/h`, name]
                  }
                  labelFormatter={(h) => `${h}:00`}
                  position={{ y: -55 }}
                  wrapperStyle={{ fontSize: 10, zIndex: 9999 }}
                />
                <Legend wrapperStyle={{ fontSize: 10 }} />
                <Line
                  yAxisId="temp"
                  type="monotone"
                  dataKey="temperature_c"
                  name={t("chart.temp")}
                  stroke={CHART_TEMP_COLOR}
                  dot={false}
                  strokeWidth={2}
                />
                <Bar
                  yAxisId="wind"
                  dataKey="wind_speed_kmh"
                  name={t("chart.wind")}
                  fill={CHART_WIND_COLOR}
                  opacity={0.6}
                />
              </ComposedChart>
            </ResponsiveContainer>
          </div>
        )}
        {avalanche ? (
          <div className={`rounded-lg px-3 py-2 text-sm ${RISK_BG[avalanche.risk_level] ?? "bg-gray-100"}`}>
            <span className="font-semibold">{t("avalanche")}: </span>
            <span className={`font-bold ${RISK_COLORS[avalanche.risk_level] ?? ""}`}>
              {avalanche.risk_label} ({avalanche.risk_level}/5)
            </span>
            {avalanche.massif_name && (
              <p className="text-xs mt-1 opacity-70">{t("massif")}: {avalanche.massif_name}</p>
            )}
            <p className="text-xs mt-1 opacity-80">{avalanche.description}</p>
            {avalanche.massif_id && avalanche.massif_id > 0 && (
              <div className="mt-2 space-y-2 overflow-y-auto max-h-96">
                {(["montagne-risques", "apercu-meteo", "sept-derniers-jours"] as const).map((imgType) => (
                  <img
                    key={imgType}
                    src={`/api/avalanche/image?massif_id=${avalanche.massif_id}&type=${imgType}`}
                    alt={imgType}
                    className="w-full rounded object-contain"
                  />
                ))}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
