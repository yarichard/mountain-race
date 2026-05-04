"use client";

import { useTranslations } from "next-intl";
import { useState, useEffect } from "react";
import dynamic from "next/dynamic";
import ReactMarkdown from "react-markdown";
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";
import type { RouteDetail } from "@/lib/types";
import { LoadingSpinner } from "./LoadingSpinner";

// Leaflet must be loaded client-side only
const MapView = dynamic(() => import("./MapView"), { ssr: false });

interface Props {
  route: RouteDetail | null;
  loading?: boolean;
}

type Tab = "map" | "elevation" | "topo";

function makeSyntheticProfile(elevGain: number) {
  const points = 20;
  return Array.from({ length: points }, (_, i) => {
    const t = i / (points - 1);
    return { dist: +(t * 10).toFixed(1), elevation: Math.round(1000 + Math.sin(t * Math.PI) * elevGain) };
  });
}

export function DetailPanel({ route, loading }: Props) {
  const t = useTranslations("detail");
  const [tab, setTab] = useState<Tab>("topo");

  useEffect(() => {
    if (route) setTab("topo");
  }, [route?.id]);

  if (!route) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className="panel-body flex-1 flex items-center justify-center text-sm text-[var(--text-muted)] text-center">
          {loading ? <LoadingSpinner message={t("loading")} /> : t("empty")}
        </div>
      </div>
    );
  }

  const isSyntheticElev = !route.elevation_profile || route.elevation_profile.length < 2;
  const elevData = isSyntheticElev
    ? makeSyntheticProfile(route.elevation_gain)
    : route.elevation_profile!.map(([dist, elev]) => ({ dist: +dist.toFixed(2), elevation: Math.round(elev) }));

  const tabs: { key: Tab; label: string }[] = [
    { key: "topo", label: t("topo") },
    { key: "map", label: t("map") },
    { key: "elevation", label: t("elevation") },
  ];

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header flex items-center justify-between">
        <span>{route.title}</span>
        <span className="text-xs bg-white/20 rounded px-2 py-0.5">{route.difficulty}</span>
      </div>
      {/* Tab bar */}
      <div className="flex border-b border-[var(--border)]">
        {tabs.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-3 py-1.5 text-xs font-semibold border-b-2 transition ${
              tab === key
                ? "border-[var(--primary)] text-[var(--primary)]"
                : "border-transparent text-[var(--text-muted)] hover:text-[var(--primary)]"
            }`}
          >
            {label}
          </button>
        ))}
        <a
          href={route.source_url}
          target="_blank"
          rel="noopener noreferrer"
          className="ml-auto px-3 py-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--primary)] transition"
        >
          {t("viewOnC2C")} ↗
        </a>
      </div>
      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {tab === "topo" && (
          <div className="panel-body">
            {route.pitches && route.pitches.length > 0 ? (
              <table className="w-full text-sm border-collapse">
                <thead>
                  <tr className="text-[var(--text-muted)] text-xs">
                    <th className="text-left py-1 pr-2">#</th>
                    <th className="text-left py-1 pr-2">{t("grade")}</th>
                    <th className="text-left py-1">{t("description")}</th>
                  </tr>
                </thead>
                <tbody>
                  {route.pitches.map((p) => (
                    <tr key={p.number} className="border-t border-[var(--border)]">
                      <td className="py-1 pr-2 font-bold text-[var(--primary)]">{p.number}</td>
                      <td className="py-1 pr-2">
                        <span className="bg-[var(--primary)] text-white text-xs rounded px-1.5 py-0.5">{p.grade}</span>
                      </td>
                      <td className="py-1 text-sm">{p.description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="text-sm leading-relaxed prose prose-sm max-w-none">
                <ReactMarkdown>{route.description}</ReactMarkdown>
              </div>
            )}
            {route.images && route.images.length > 0 && (
              <div className="flex gap-2 mt-3 overflow-x-auto pb-1">
                {route.images.map((name) => (
                  <a
                    key={name}
                    href={`/api/images?source=CampToCamp&name=${encodeURIComponent(name)}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="shrink-0"
                  >
                    <img
                      src={`/api/images?source=CampToCamp&name=${encodeURIComponent(name)}`}
                      alt=""
                      className="h-32 w-auto rounded object-cover"
                    />
                  </a>
                ))}
              </div>
            )}
          </div>
        )}
        {tab === "map" && (
          <div className="h-64">
            <MapView lat={route.lat || 45.9} lon={route.lon || 6.9} track={route.track} />
          </div>
        )}
        {tab === "elevation" && (
          <div className="panel-body">
            <ResponsiveContainer width="100%" height={180}>
              <AreaChart data={elevData} margin={{ top: 8, right: 8, bottom: 0, left: 30 }}>
                <defs>
                  <linearGradient id="elevGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#1F2782" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#1F2782" stopOpacity={0.05} />
                  </linearGradient>
                </defs>
                <XAxis dataKey="dist" tick={{ fontSize: 10 }} tickFormatter={(v) => `${v}km`} />
                <YAxis tick={{ fontSize: 10 }} tickFormatter={(v) => `${v}m`} />
                <Tooltip formatter={(v) => [`${v} m`, "Altitude"]} labelFormatter={(l) => `${l} km`} />
                <Area type="monotone" dataKey="elevation" stroke="#1F2782" fill="url(#elevGrad)" strokeWidth={2} dot={false} />
              </AreaChart>
            </ResponsiveContainer>
            {isSyntheticElev && (
              <p className="text-xs text-[var(--text-muted)] mt-1 text-center italic">{t("elevationEstimated")}</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
