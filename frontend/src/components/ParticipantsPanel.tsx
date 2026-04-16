"use client";

import { useTranslations } from "next-intl";
import { CLIMBING_LEVELS, type Participant } from "@/lib/types";

interface Props {
  participants: Participant[];
  onChange: (p: Participant[]) => void;
}

export function ParticipantsPanel({ participants, onChange }: Props) {
  const t = useTranslations("participants");

  const add = () =>
    onChange([...participants, { name: "", climbingLevel: "5c" }]);

  const remove = (i: number) =>
    onChange(participants.filter((_, idx) => idx !== i));

  const update = (i: number, field: keyof Participant, value: string) => {
    const next = [...participants];
    next[i] = { ...next[i], [field]: value };
    onChange(next);
  };

  return (
    <div className="panel flex flex-col h-full">
      <div className="panel-header">{t("title")}</div>
      <div className="panel-body flex-1 overflow-y-auto space-y-2">
        {participants.map((p, i) => (
          <div key={i} className="flex gap-1 items-center">
            <input
              className="flex-1 min-w-0 border border-[var(--border)] rounded px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
              placeholder={t("name")}
              value={p.name}
              onChange={(e) => update(i, "name", e.target.value)}
            />
            <select
              className="border border-[var(--border)] rounded px-1 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-[var(--primary)]"
              value={p.climbingLevel}
              onChange={(e) => update(i, "climbingLevel", e.target.value)}
            >
              {CLIMBING_LEVELS.map((g) => (
                <option key={g} value={g}>{g}</option>
              ))}
            </select>
            <button
              onClick={() => remove(i)}
              className="text-red-400 hover:text-red-600 text-lg leading-none px-1"
              title={t("remove")}
            >
              ×
            </button>
          </div>
        ))}
        <button
          onClick={add}
          className="mt-1 w-full py-1 rounded border border-dashed border-[var(--primary)] text-[var(--primary)] text-sm hover:bg-[var(--surface-alt)] transition"
        >
          + {t("add")}
        </button>
      </div>
    </div>
  );
}
