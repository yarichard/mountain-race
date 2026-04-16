import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";
import { IntlProvider } from "@/components/IntlProvider";

const geist = Geist({ subsets: ["latin"], variable: "--font-geist" });

export const metadata: Metadata = {
  title: "Mountain Race",
  description: "Plan your mountain adventure",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="fr" className={`${geist.variable} h-full`}>
      <body className="min-h-full antialiased">
        <IntlProvider>{children}</IntlProvider>
      </body>
    </html>
  );
}
