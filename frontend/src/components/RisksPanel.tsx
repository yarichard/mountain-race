"use client";

import { useTranslations } from "next-intl";
import type { RouteDetail } from "@/lib/types";

interface Props {
  route: RouteDetail | null;
}

export function RisksPanel({ route }: Props) {
  const t = useTranslations("risks");

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

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 overflow-y-auto">
        <ul className="space-y-2">
          {route.risks.map((risk, i) => (
            <li key={i} className="flex gap-2 text-sm">
              <span className="mt-0.5 text-orange-500">⚠</span>
              <span>{risk}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
