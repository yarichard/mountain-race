"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { MULTIPITCH_GRADES, ALPINE_GRADES, type RaceType, type RouteResult, type Participant } from "@/lib/types";

interface Props {
  participants: Participant[];
  onRouteSelected: (id: string, date: string) => void;
  onWeatherInvalidated?: () => void;
  onDateChange?: (date: string) => void;
}

export function SearchPanel({ participants, onRouteSelected, onWeatherInvalidated, onDateChange }: Props) {
  const t = useTranslations("search");

  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [raceType, setRaceType] = useState<RaceType>("multipitch");
  const [difficulty, setDifficulty] = useState("5c");
  const [location, setLocation] = useState("");
  const [locationType, setLocationType] = useState<"name" | "location">("name");
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<RouteResult[] | null>(null);

  const grades = raceType === "multipitch" ? MULTIPITCH_GRADES : ALPINE_GRADES;

  const handleRaceTypeChange = (t: RaceType) => {
    setRaceType(t);
    setDifficulty(t === "multipitch" ? "5c" : "AD");
  };

  const search = async () => {
    onWeatherInvalidated?.();
    setSearching(true);
    setResults(null);
    try {
      const res = await fetch("/api/routes/search", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          location,
          location_type: locationType,
          race_type: raceType,
          difficulty,
          date,
          participants: participants.map((p) => ({
            name: p.name,
            climbing_level: p.climbingLevel,
          })),
        }),
      });
      const data = await res.json();
      setResults(data.routes ?? []);
    } catch {
      setResults([]);
    } finally {
      setSearching(false);
    }
  };

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 overflow-y-auto space-y-2">
        {/* Date */}
        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-0.5">{t("date")}</label>
          <input
            type="date"
            className="w-full border border-[var(--border)] rounded px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
            value={date}
            onChange={(e) => { setDate(e.target.value); onDateChange?.(e.target.value); }}
          />
        </div>

        {/* Race type */}
        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-0.5">{t("raceType")}</label>
          <div className="flex gap-1">
            {(["multipitch", "ridge_hike", "hike"] as RaceType[]).map((rt) => (
              <button
                key={rt}
                onClick={() => handleRaceTypeChange(rt)}
                className={`flex-1 text-xs py-1 rounded border transition ${
                  raceType === rt
                    ? "bg-[var(--primary)] text-white border-[var(--primary)]"
                    : "bg-white text-[var(--primary)] border-[var(--border)] hover:border-[var(--primary)]"
                }`}
              >
                {t(`raceTypes.${rt}` as Parameters<typeof t>[0])}
              </button>
            ))}
          </div>
        </div>

        {/* Difficulty */}
        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-0.5">{t("difficulty")}</label>
          <select
            className="w-full border border-[var(--border)] rounded px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
            value={difficulty}
            onChange={(e) => setDifficulty(e.target.value)}
          >
            {grades.map((g) => (
              <option key={g} value={g}>{g}</option>
            ))}
          </select>
        </div>

        {/* Location */}
        <div>
          <div className="flex items-center justify-between mb-0.5">
            <label className="text-xs font-semibold text-[var(--text-muted)]">{t("location")}</label>
            <div className="flex rounded border border-[var(--border)] overflow-hidden text-xs">
              {(["name", "location"] as const).map((mode) => (
                <button
                  key={mode}
                  onClick={() => setLocationType(mode)}
                  className={`px-2 py-0.5 transition ${
                    locationType === mode
                      ? "bg-[var(--primary)] text-white"
                      : "bg-white text-[var(--text-muted)] hover:text-[var(--primary)]"
                  }`}
                >
                  {t(`locationMode.${mode}` as Parameters<typeof t>[0])}
                </button>
              ))}
            </div>
          </div>
          <input
            className="w-full border border-[var(--border)] rounded px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
            placeholder={locationType === "location" ? t("locationPlaceholderGeo") : t("locationPlaceholder")}
            value={location}
            onChange={(e) => setLocation(e.target.value)}
          />
        </div>

        {/* Search button */}
        <button
          onClick={search}
          disabled={searching}
          className="w-full bg-[var(--primary)] hover:bg-[var(--primary-light)] text-white font-semibold text-sm py-2 rounded transition disabled:opacity-60"
        >
          {searching ? t("searching") : t("search")}
        </button>

        {/* Results */}
        {results !== null && (
          <div>
            <p className="text-xs font-semibold text-[var(--text-muted)] mb-1">{t("results")}</p>
            {results.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("noResults")}</p>
            ) : (
              <div className="space-y-1">
                {results.map((r) => (
                  <div
                    key={r.id}
                    data-testid="route-result"
                    className="border border-[var(--border)] rounded p-2 hover:border-[var(--primary)] hover:bg-[var(--surface-alt)] cursor-pointer transition"
                    onClick={() => { onRouteSelected(r.id, date); setResults(null); }}
                  >
                    <div className="flex justify-between items-start gap-2">
                      <p className="text-sm font-medium leading-tight flex-1">{r.title}</p>
                      <span className="shrink-0 text-xs font-bold bg-[var(--primary)] text-white rounded px-1.5 py-0.5">{r.difficulty}</span>
                    </div>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5">
                      ↑{r.elevation_gain}m · {r.distance_km.toFixed(1)}km
                    </p>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
