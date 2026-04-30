"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import type { Equipment, RouteDetail } from "@/lib/types";
import { LoadingSpinner } from "./LoadingSpinner";

interface Props {
  route: RouteDetail | null;
  equipment: Equipment[] | null; // null = loading, [] = empty/failed
  gearText?: string;             // shown when LLM extraction failed
}

export function EquipmentPanel({ route, equipment, gearText }: Props) {
  const t = useTranslations("equipment");
  const [showOriginal, setShowOriginal] = useState(true);

  if (!route) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className="panel-body flex-1 flex items-center justify-center text-sm text-[var(--text-muted)] text-center">
          {t("empty")}
        </div>
      </div>
    );
  }

  if (equipment === null) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className="panel-body flex-1 flex items-center justify-center">
          <LoadingSpinner message={t("loading")} />
        </div>
      </div>
    );
  }

  if (equipment.length === 0 && gearText) {
    return (
      <div className="panel flex flex-col h-full">
        <div className="panel-header">{t("title")}</div>
        <div className="panel-body flex-1 overflow-y-auto">
          <p className="text-xs text-[var(--text-muted)] italic mb-2">{t("extractionFailed")}</p>
          <p className="text-sm whitespace-pre-wrap">{gearText}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header flex items-center justify-between">
        <span>{t("title")}</span>
      </div>
      <div className="panel-body flex-1 overflow-y-auto">
        {gearText && (
          <label className="flex items-center gap-1 text-xs font-normal cursor-pointer">
            <input
              type="checkbox"
              checked={showOriginal}
              onChange={(e) => setShowOriginal(e.target.checked)}
              className="cursor-pointer"
            />
            {t("showOriginal")}
          </label>
        )}
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="text-[var(--text-muted)] text-xs border-b border-[var(--border)]">
              <th className="text-left py-1 pr-2">{t("item")}</th>
              <th className="text-center py-1 pr-2">{t("quantity")}</th>
              <th className="text-left py-1">{t("notes")}</th>
            </tr>
          </thead>
          <tbody>
            {equipment.map((eq, i) => (
              <tr key={i} className="border-t border-[var(--border)]">
                <td className="py-1 pr-2 font-medium">{eq.item}</td>
                <td className="py-1 pr-2 text-center text-[var(--primary)] font-bold">{eq.quantity}</td>
                <td className="py-1 text-[var(--text-muted)] text-xs">{eq.notes}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {showOriginal && gearText && (
          <div className="mt-3 pt-3 border-t border-[var(--border)]">
            <p className="text-xs text-[var(--text-muted)] font-medium mb-1">{t("originalLabel")}</p>
            <p className="text-xs whitespace-pre-wrap text-[var(--text-muted)]">{gearText}</p>
          </div>
        )}
      </div>
    </div>
  );
}
