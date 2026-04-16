"use client";

import { useTranslations } from "next-intl";
import type { RouteDetail } from "@/lib/types";

interface Props {
  route: RouteDetail | null;
}

export function EquipmentPanel({ route }: Props) {
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
            {route.equipment.map((eq, i) => (
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
