import "./globals.css";
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { Providers } from "@/components/providers";
import { THEME_BOOT_SCRIPT } from "@/lib/theme";

const inter = Inter({ subsets: ["latin"], display: "swap" });

export const metadata: Metadata = {
  title: {
    default: "TheDobra — Analytics nativo em IA",
    template: "%s · TheDobra",
  },
  description: "Ligue os dados. Entenda o negócio. Aja com inteligência.",
  applicationName: "TheDobra",
  metadataBase: new URL(process.env.NEXT_PUBLIC_APP_URL || "http://localhost:3010"),
  icons: {
    icon: [{ url: "/logo-mark.svg", type: "image/svg+xml" }],
    shortcut: "/logo-mark.svg",
    apple: "/logo-mark.svg",
  },
  openGraph: {
    title: "TheDobra — Analytics nativo em IA",
    description: "Ligue os dados. Entenda o negócio. Aja com inteligência.",
    siteName: "TheDobra",
    images: ["/logo-thedobra.png"],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR" suppressHydrationWarning>
      <body className={inter.className}>
        <script dangerouslySetInnerHTML={{ __html: THEME_BOOT_SCRIPT }} />
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
