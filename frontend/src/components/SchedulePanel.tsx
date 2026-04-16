"use client";

import { useTranslations } from "next-intl";
import type { RouteDetail } from "@/lib/types";

interface Props {
  route: RouteDetail | null;
}

export function SchedulePanel({ route }: Props) {
  const t = useTranslations("schedule");

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

  const { schedule } = route;

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 space-y-2">
        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
          <span className="text-[var(--text-muted)]">{t("duration")}</span>
          <span className="font-semibold text-[var(--primary)]">
            {schedule.estimated_duration_hours.toFixed(1)} {t("hours")}
          </span>
          <span className="text-[var(--text-muted)]">{t("start")}</span>
          <span className="font-semibold">{schedule.recommended_start_time}</span>
          <span className="text-[var(--text-muted)]">{t("end")}</span>
          <span className="font-semibold">{schedule.recommended_end_time}</span>
        </div>
        {schedule.source === "formula" && (
          <div className="mt-2 bg-amber-50 border border-amber-200 rounded p-2 text-xs text-amber-800">
            ℹ️ {t("naissmithNotice")}
          </div>
        )}
      </div>
    </div>
  );
}
