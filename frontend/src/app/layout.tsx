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
    metadataBase: new URL("https://happyfeet.example"),
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
