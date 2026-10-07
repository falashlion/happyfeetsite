import "@/styles/globals.css";
import type { Metadata, Viewport } from "next";
import { Header, type NavLink } from "@/components/header";
import { listCategoriesApi } from "@/lib/api/products";
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

  // Resolved here rather than in the header so the real categories are in the
  // server-rendered HTML: fetching after hydration showed the fallback first,
  // then swapped, and left crawlers with links to the demo categories.
  // Categories with no products are skipped — a link to an empty one is a dead
  // end for a shopper.
  const navCategories: NavLink[] = (await listCategoriesApi())
    .filter((c) => c.productCount > 0)
    .map((c) => ({ slug: c.slug, label: c.name }));

  return (
    <html lang={locale}>
      <body>
        <Providers locale={locale}>
          <Header categories={navCategories} />
          <main style={{ flex: 1 }}>{children}</main>
          <Footer />
          <WhatsAppFab />
        </Providers>
      </body>
    </html>
  );
}
