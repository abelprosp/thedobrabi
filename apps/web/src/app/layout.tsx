import "./globals.css";
import type { Metadata, Viewport } from "next";
import { Inter } from "next/font/google";
import { Providers } from "@/components/providers";
import { THEME_BOOT_SCRIPT } from "@/lib/theme";

const inter = Inter({ subsets: ["latin"], display: "swap" });

// Base para og:image/twitter:image. Em produção nunca cair em http://localhost
// (aconteceu quando o build corre sem NEXT_PUBLIC_APP_URL): a página HTTPS
// passaria a anunciar URLs http:// absolutas nas meta tags.
function resolveMetadataBase(): URL {
  const fromEnv = process.env.NEXT_PUBLIC_APP_URL || process.env.APP_PUBLIC_URL;
  if (fromEnv) return new URL(fromEnv);
  return new URL(process.env.NODE_ENV === "production" ? "https://app.thedobra.cc" : "http://localhost:3010");
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  viewportFit: "cover",
};

export const metadata: Metadata = {
  title: {
    default: "TheDobra — Analytics nativo em IA",
    template: "%s · TheDobra",
  },
  description: "Ligue os dados. Entenda o negócio. Aja com inteligência.",
  applicationName: "TheDobra",
  metadataBase: resolveMetadataBase(),
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
