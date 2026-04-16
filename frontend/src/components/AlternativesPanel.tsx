"use client";

import { useTranslations } from "next-intl";
import type { RouteDetail } from "@/lib/types";

interface Props {
  route: RouteDetail | null;
}

export function AlternativesPanel({ route }: Props) {
  const t = useTranslations("alternatives");

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
      <div className="panel-body flex-1 overflow-y-auto space-y-2">
        {route.alternative_routes.length === 0 ? (
          <p className="text-sm text-[var(--text-muted)]">{t("empty")}</p>
        ) : (
          route.alternative_routes.map((alt) => (
            <div key={alt.id} className="border border-[var(--border)] rounded p-2">
              <div className="flex justify-between items-start gap-2">
                <p className="text-sm font-medium">{alt.title}</p>
                <a
                  href={`https://www.camptocamp.org/routes/${alt.id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="shrink-0 text-xs text-[var(--primary)] hover:underline"
                >
                  {t("viewOnC2C")} ↗
                </a>
              </div>
              {alt.reason && (
                <p className="text-xs text-[var(--text-muted)] mt-0.5">{alt.reason}</p>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
