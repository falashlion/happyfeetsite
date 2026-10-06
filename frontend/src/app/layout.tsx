import "@/styles/globals.css";
import type { Metadata, Viewport } from "next";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { WhatsAppFab } from "@/components/whatsapp-fab";
import { Providers } from "@/components/providers";
import { getLocale, tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return {
    title: {
      default: `${t("common.brand")} — ${t("common.tagline")}`,
      template: `%s · ${t("common.brand")}`,
    },
    description: t("footer.about_blurb"),
    // Absolute URLs for Open Graph and icons are resolved against this. The
    // placeholder it replaced meant every shared link advertised a domain that
    // does not exist. NEXT_PUBLIC_SITE_URL is inlined at build time; the
    // fallback keeps local development working.
    metadataBase: new URL(
      process.env.NEXT_PUBLIC_SITE_URL?.trim() || "http://localhost:3000",
    ),
    openGraph: {
      title: t("common.brand"),
      description: t("common.tagline"),
      type: "website",
    },
  };
}

export const viewport: Viewport = {
  themeColor: "#0E1B3A",
  width: "device-width",
  initialScale: 1,
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const locale = await getLocale();
  return (
    <html lang={locale}>
      <body>
        <Providers locale={locale}>
          <Header />
          <main style={{ flex: 1 }}>{children}</main>
          <Footer />
          <WhatsAppFab />
        </Providers>
      </body>
    </html>
  );
}
