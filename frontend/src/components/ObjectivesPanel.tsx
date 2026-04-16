"use client";

import { useTranslations } from "next-intl";

interface Props {
  objectives: string[];
  notes: string;
  onObjectivesChange: (o: string[]) => void;
  onNotesChange: (n: string) => void;
}

const KEYS = ["challenge", "fun", "performance", "discovery"] as const;

export function ObjectivesPanel({ objectives, notes, onObjectivesChange, onNotesChange }: Props) {
  const t = useTranslations("objectives");

  const toggle = (key: string) => {
    if (objectives.includes(key)) {
      onObjectivesChange(objectives.filter((o) => o !== key));
    } else {
      onObjectivesChange([...objectives, key]);
    }
  };

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 space-y-2">
        <div className="flex flex-wrap gap-2">
          {KEYS.map((k) => (
            <button
              key={k}
              onClick={() => toggle(k)}
              className={`px-3 py-1 rounded-full text-xs font-semibold border transition ${
                objectives.includes(k)
                  ? "bg-[var(--primary)] text-white border-[var(--primary)]"
                  : "bg-white text-[var(--primary)] border-[var(--border)] hover:border-[var(--primary)]"
              }`}
            >
              {t(k)}
            </button>
          ))}
        </div>
        <textarea
          className="w-full border border-[var(--border)] rounded px-2 py-1 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
          rows={3}
          placeholder={t("notes")}
          value={notes}
          onChange={(e) => onNotesChange(e.target.value)}
        />
      </div>
    </div>
  );
}
