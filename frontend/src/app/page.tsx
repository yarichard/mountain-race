"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { ParticipantsPanel } from "@/components/ParticipantsPanel";
import { ObjectivesPanel } from "@/components/ObjectivesPanel";
import { WeatherPanel } from "@/components/WeatherPanel";
import { SearchPanel } from "@/components/SearchPanel";
import { DetailPanel } from "@/components/DetailPanel";
import { RisksPanel } from "@/components/RisksPanel";
import { AlternativesPanel } from "@/components/AlternativesPanel";
import { SchedulePanel } from "@/components/SchedulePanel";
import { EquipmentPanel } from "@/components/EquipmentPanel";
import type { Participant, RouteDetail, WeatherData } from "@/lib/types";

export default function Home() {
  const t = useTranslations();

  const [participants, setParticipants] = useState<Participant[]>([
    { name: "", climbingLevel: "5c" },
  ]);
  const [objectives, setObjectives] = useState<string[]>([]);
  const [notes, setNotes] = useState("");
  const [route, setRoute] = useState<RouteDetail | null>(null);
  const [weather, setWeather] = useState<WeatherData | null>(null);
  const [weatherError, setWeatherError] = useState(false);
  const [exporting, setExporting] = useState(false);

  const handleRouteSelected = async (id: string, date: string) => {
    const routeRes = await fetch(`/api/routes/${id}`);
    if (!routeRes.ok) return;
    const routeData = await routeRes.json();
    setRoute(routeData);

    setWeather(null);
    setWeatherError(false);
    const lat = routeData.lat ?? 45.9;
    const lon = routeData.lon ?? 6.9;
    const weatherRes = await fetch(`/api/weather?lat=${lat}&lon=${lon}&date=${date}`);
    if (weatherRes.ok) {
      setWeather(await weatherRes.json());
    } else {
      setWeatherError(true);
    }
  };

  const handleExport = async () => {
    if (!route) return;
    setExporting(true);
    try {
      const res = await fetch("/api/export/pdf", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...route, weather: weather ?? {} }),
      });
      if (!res.ok) throw new Error("export failed");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "mountain-race.pdf";
      a.click();
      URL.revokeObjectURL(url);
    } finally {
      setExporting(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col bg-[var(--surface-alt)]">
      {/* ── Header ── */}
      <header className="flex items-center justify-between px-5 py-2.5 bg-[var(--primary)] shadow-md z-10">
        <div className="flex items-center gap-3">
          <span className="text-2xl select-none">🏔️</span>
          <h1 className="text-white font-bold text-base tracking-widest uppercase">
            {t("app.title")}
          </h1>
        </div>
        <button
          onClick={handleExport}
          disabled={!route || exporting}
          className="flex items-center gap-2 bg-white text-[var(--primary)] font-semibold text-sm px-4 py-1.5 rounded-full shadow hover:bg-blue-50 transition disabled:opacity-40 disabled:cursor-not-allowed"
        >
          📄 {exporting ? t("export.exporting") : t("export.button")}
        </button>
      </header>

      {/*
        ── 9-panel layout ──
        Grid areas:
          p1   | p2 p4    | p3
          p1   | p5       | p6
          p9   | p5       | p6
          --   | p8  p7   | p6

        Columns: 210px | 1fr | 270px
      */}
      <main
        className="flex-1 p-2 gap-2 overflow-hidden"
        style={{
          display: "grid",
          gridTemplateColumns: "210px 1fr 270px",
          gridTemplateRows: "auto 1fr auto auto",
          gridTemplateAreas: `
            "p1  top-mid   p3"
            "p1  p5        p6"
            "p9  p5        p6"
            ".   bot-mid   p6"
          `,
          minHeight: 0,
        }}
      >
        {/* Part 1: Participants — col 1, rows 1-3 */}
        <div style={{ gridArea: "p1" }} className="min-h-0">
          <ParticipantsPanel participants={participants} onChange={setParticipants} />
        </div>

        {/* Part 9: Equipment — col 1, row 4 */}
        <div style={{ gridArea: "p9" }} className="min-h-0">
          <EquipmentPanel route={route} />
        </div>

        {/* Top-mid: Part 2 + Part 4 side-by-side */}
        <div style={{ gridArea: "top-mid" }} className="grid grid-cols-[160px_1fr] gap-2 min-h-0">
          <ObjectivesPanel
            objectives={objectives}
            notes={notes}
            onObjectivesChange={setObjectives}
            onNotesChange={setNotes}
          />
          <SearchPanel participants={participants} onRouteSelected={handleRouteSelected} />
        </div>

        {/* Part 5: Race detail — col 2, rows 2-3 */}
        <div style={{ gridArea: "p5" }} className="min-h-0">
          <DetailPanel route={route} />
        </div>

        {/* Bottom-mid: Part 8 + Part 7 */}
        <div style={{ gridArea: "bot-mid" }} className="grid grid-cols-2 gap-2 min-h-0">
          <SchedulePanel route={route} />
          <AlternativesPanel route={route} />
        </div>

        {/* Part 3: Weather — col 3, row 1 */}
        <div style={{ gridArea: "p3" }} className="min-h-0">
          <WeatherPanel weather={weather} error={weatherError} />
        </div>

        {/* Part 6: Risks — col 3, rows 2-4 */}
        <div style={{ gridArea: "p6" }} className="min-h-0">
          <RisksPanel route={route} />
        </div>
      </main>
    </div>
  );
}
