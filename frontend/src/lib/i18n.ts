import fr from "../../messages/fr.json";
import en from "../../messages/en.json";

export type Locale = "fr" | "en";

export const messages = { fr, en } as const;

export function detectLocale(): Locale {
  if (typeof navigator === "undefined") return "fr";
  const lang = navigator.language.toLowerCase();
  if (lang.startsWith("fr")) return "fr";
  return "en";
}
