"use client";

import { useState, useRef } from "react";
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
import type { Equipment, Participant, RouteDetail, WeatherData } from "@/lib/types";

export default function Home() {
  const t = useTranslations();

  const [participants, setParticipants] = useState<Participant[]>([
    { name: "", climbingLevel: "5c" },
  ]);
  const [objectives, setObjectives] = useState<string[]>([]);
  const [notes, setNotes] = useState("");
  const [route, setRoute] = useState<RouteDetail | null>(null);
  const [loadingRoute, setLoadingRoute] = useState(false);
  const [equipment, setEquipment] = useState<Equipment[] | null>(null);
  const [gearText, setGearText] = useState<string | undefined>(undefined);
  const [weather, setWeather] = useState<WeatherData | null>(null);
  const [loadingWeather, setLoadingWeather] = useState(false);
  const [weatherError, setWeatherError] = useState(false);
  const [exporting, setExporting] = useState(false);
  // true initially so the first route selection always fetches weather
  const weatherStale = useRef(true);

  const fetchWeatherForRoute = async (lat: number, lon: number, date: string) => {
    setWeather(null);
    setWeatherError(false);
    setLoadingWeather(true);
    const res = await fetch(`/api/weather?lat=${lat}&lon=${lon}&date=${date}`);
    setLoadingWeather(false);
    if (res.ok) setWeather(await res.json());
    else setWeatherError(true);
  };

  // Called when the search date changes: refresh weather immediately if a route is already selected
  const handleDateChange = (date: string, currentRoute: typeof route) => {
    if (currentRoute) {
      void fetchWeatherForRoute(currentRoute.lat ?? 45.9, currentRoute.lon ?? 6.9, date);
    }
  };

  const fetchEquipment = async (gearTextValue: string) => {
    setEquipment(null);
    const res = await fetch("/api/equipment/extract", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ gear_text: gearTextValue }),
    });
    if (res.ok) {
      const data = await res.json();
      setEquipment(data.equipment ?? []);
    } else {
      setEquipment([]);
    }
  };

  // Called when a new search is launched: clear route (Part 5) and weather (Part 3)
  const handleWeatherInvalidated = () => {
    setRoute(null);
    setLoadingRoute(false);
    setWeather(null);
    setWeatherError(false);
    setLoadingWeather(false);
    setEquipment(null);
    setGearText(undefined);
    weatherStale.current = true;
  };

  const handleRouteSelected = async (id: string, date: string) => {
    setRoute(null);
    setEquipment(null);
    setGearText(undefined);
    setLoadingRoute(true);
    const routeRes = await fetch(`/api/routes/${id}`);
    if (!routeRes.ok) { setLoadingRoute(false); return; }
    const routeData: RouteDetail = await routeRes.json();
    setRoute(routeData);
    setLoadingRoute(false);

    // Fire gear extraction independently — does not block route display
    const gt = routeData.gear_text ?? "";
    setGearText(gt);
    void fetchEquipment(gt);

    if (!weatherStale.current) return;
    weatherStale.current = false;
    void fetchWeatherForRoute(routeData.lat ?? 45.9, routeData.lon ?? 6.9, date);
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
          gridTemplateColumns: "210px 1fr 350px",
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
          <EquipmentPanel route={route} equipment={equipment} gearText={gearText} />
        </div>

        {/* Top-mid: Part 2 + Part 4 side-by-side */}
        <div style={{ gridArea: "top-mid" }} className="grid grid-cols-[160px_1fr] gap-2 min-h-0">
          <ObjectivesPanel
            objectives={objectives}
            notes={notes}
            onObjectivesChange={setObjectives}
            onNotesChange={setNotes}
          />
          <SearchPanel
            participants={participants}
            onRouteSelected={handleRouteSelected}
            onWeatherInvalidated={handleWeatherInvalidated}
            objectives={objectives}
            onDateChange={(date) => handleDateChange(date, route)}
          />
        </div>

        {/* Part 5: Race detail — col 2, rows 2-3 */}
        <div style={{ gridArea: "p5" }} className="min-h-0">
          <DetailPanel route={route} loading={loadingRoute} />
        </div>

        {/* Bottom-mid: Part 8 + Part 7 */}
        <div style={{ gridArea: "bot-mid" }} className="grid grid-cols-2 gap-2 min-h-0">
          <SchedulePanel route={route} />
          <AlternativesPanel route={route} />
        </div>

        {/* Part 3: Weather — col 3, row 1 */}
        <div style={{ gridArea: "p3" }} className="min-h-0">
          <WeatherPanel weather={weather} loading={loadingWeather} error={weatherError} />
        </div>

        {/* Part 6: Risks — col 3, rows 2-4 */}
        <div style={{ gridArea: "p6" }} className="min-h-0">
          <RisksPanel route={route} />
        </div>
      </main>
    </div>
  );
}
