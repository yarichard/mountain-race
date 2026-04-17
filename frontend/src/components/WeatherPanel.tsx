"use client";

import { useTranslations } from "next-intl";
import type { WeatherData } from "@/lib/types";

interface Props {
  weather: WeatherData | null;
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

export function WeatherPanel({ weather, error }: Props) {
  const t = useTranslations("weather");

  if (!weather) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className={`panel-body flex-1 flex items-center justify-center text-sm text-center ${error ? "text-red-600" : "text-[var(--text-muted)]"}`}>
          {error ? t("error") : t("empty")}
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
        <div className={`rounded-lg px-3 py-2 text-sm ${RISK_BG[avalanche.risk_level] ?? "bg-gray-100"}`}>
          <span className="font-semibold">{t("avalanche")}: </span>
          <span className={`font-bold ${RISK_COLORS[avalanche.risk_level] ?? ""}`}>
            {avalanche.risk_label} ({avalanche.risk_level}/5)
          </span>
          <p className="text-xs mt-1 opacity-80">{avalanche.description}</p>
        </div>
      </div>
    </div>
  );
}
