"use client";

import { NextIntlClientProvider } from "next-intl";
import { useEffect, useState } from "react";
import { detectLocale, messages, type Locale } from "@/lib/i18n";

export function IntlProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocale] = useState<Locale>("fr");

  useEffect(() => {
    setLocale(detectLocale());
  }, []);

  return (
    <NextIntlClientProvider locale={locale} messages={messages[locale]}>
      {children}
    </NextIntlClientProvider>
  );
}
