"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import {
  MULTIPITCH_GRADES,
  ALPINE_GRADES,
  ALPINE_TO_CLIMBING,
  CLIMBING_LEVELS,
  type RaceType,
  type RouteResult,
  type Participant,
} from "@/lib/types";

interface Props {
  participants: Participant[];
  objectives: string[];
  onRouteSelected: (id: string, date: string) => void;
  onWeatherInvalidated?: () => void;
  onDateChange?: (date: string) => void;
}

function gradeIndexInClimbing(g: string): number {
  return (CLIMBING_LEVELS as readonly string[]).indexOf(g);
}

function alpineToClimbingEquiv(alpine: string): string {
  return ALPINE_TO_CLIMBING[alpine] ?? "";
}

function gradeColor(grade: string, lowestLevel: string, raceType: RaceType): string {
  if (!lowestLevel) return "";
  const routeEquiv =
    raceType === "multipitch" ? grade : alpineToClimbingEquiv(grade);
  if (!routeEquiv) return "";
  const ri = gradeIndexInClimbing(routeEquiv);
  const pi = gradeIndexInClimbing(lowestLevel);
  if (ri < 0 || pi < 0) return "";
  if (ri < pi) return "green";
  if (ri === pi) return "black";
  return "red";
}

function badgeClass(color: string): string {
  if (color === "green") return "bg-green-600 text-white";
  if (color === "red") return "bg-red-600 text-white";
  return "bg-[var(--primary)] text-white";
}

function hasPermissiveObjective(objectives: string[]): boolean {
  return objectives.some((o) => o === "challenge" || o === "performance");
}

export function SearchPanel({ participants, objectives, onRouteSelected, onWeatherInvalidated, onDateChange }: Props) {
  const t = useTranslations("search");

  const [date, setDate] = useState(new Date().toISOString().split("T")[0]);
  const [raceType, setRaceType] = useState<RaceType>("multipitch");
  const [difficulty, setDifficulty] = useState("5c");
  const [location, setLocation] = useState("");
  const [locationType, setLocationType] = useState<"name" | "location">("name");
  const [radiusKm, setRadiusKm] = useState(20);
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<RouteResult[] | null>(null);
  const [allowAbove, setAllowAbove] = useState(() =>
    hasPermissiveObjective(objectives)
  );

  // Sync checkbox default when objectives change.
  useEffect(() => {
    setAllowAbove(hasPermissiveObjective(objectives));
  }, [objectives]);

  const allGrades = raceType === "multipitch" ? MULTIPITCH_GRADES : ALPINE_GRADES;

  // Lowest climbing level among participants with a valid level set.
  const filledParticipants = participants.filter((p) => p.name.trim() !== "");
  const lowestLevel =
    filledParticipants.length === 0
      ? ""
      : filledParticipants.reduce((min, p) => {
          const mi = gradeIndexInClimbing(min);
          const pi = gradeIndexInClimbing(p.climbingLevel);
          return pi >= 0 && (mi < 0 || pi < mi) ? p.climbingLevel : min;
        }, filledParticipants[0].climbingLevel);

  // Grades shown in the difficulty dropdown, filtered when allowAbove is false.
  const visibleGrades = allGrades.filter((g) => {
    if (!lowestLevel || allowAbove) return true;
    const color = gradeColor(g, lowestLevel, raceType);
    return color !== "red";
  });

  const handleRaceTypeChange = (rt: RaceType) => {
    setRaceType(rt);
    const defaultDiff = rt === "multipitch" ? "5c" : "AD";
    setDifficulty(defaultDiff);
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
          allow_above: allowAbove,
          date,
          radius_km: locationType === "location" ? radiusKm : undefined,
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
            {visibleGrades.map((g) => {
              const c = gradeColor(g, lowestLevel, raceType);
              return (
                <option
                  key={g}
                  value={g}
                  style={{
                    color: c === "green" ? "#16a34a" : c === "red" ? "#dc2626" : undefined,
                    fontWeight: c ? "600" : undefined,
                  }}
                >
                  {g}
                </option>
              );
            })}
          </select>

          {/* Allow-above checkbox */}
          <label className="flex items-center gap-1.5 mt-1 cursor-pointer select-none">
            <input
              type="checkbox"
              className="accent-[var(--primary)]"
              checked={allowAbove}
              onChange={(e) => setAllowAbove(e.target.checked)}
            />
            <span className="text-xs text-[var(--text-muted)]">{t("allowAbove")}</span>
          </label>
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

        {/* Radius — only for area/location mode */}
        {locationType === "location" && (
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-0.5">{t("radius")}</label>
            <input
              type="number"
              min={1}
              max={200}
              className="w-full border border-[var(--border)] rounded px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
              value={radiusKm}
              onChange={(e) => setRadiusKm(Math.max(1, parseInt(e.target.value, 10) || 1))}
            />
          </div>
        )}

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
              <div className="space-y-1 max-h-64 overflow-y-auto pr-1">
                {results.map((r) => (
                  <div
                    key={r.id}
                    data-testid="route-result"
                    className="border border-[var(--border)] rounded p-2 hover:border-[var(--primary)] hover:bg-[var(--surface-alt)] cursor-pointer transition"
                    onClick={() => { onRouteSelected(r.id, date); setResults(null); }}
                  >
                    <div className="flex justify-between items-start gap-2">
                      <p className="text-sm font-medium leading-tight flex-1">{r.title}</p>
                      <span className={`shrink-0 text-xs font-bold rounded px-1.5 py-0.5 ${badgeClass(r.difficulty_color)}`}>
                        {r.difficulty}
                      </span>
                    </div>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5">
                      <span className="text-green-600">↑{r.elevation_gain}m</span>
                      {" · "}
                      <span>{r.distance_km}km</span>
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