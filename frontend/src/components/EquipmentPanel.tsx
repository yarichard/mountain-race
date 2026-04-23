"use client";

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
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 overflow-y-auto">
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
      </div>
    </div>
  );
}
