import Link from "next/link";
import type { Metadata } from "next";
import { listCategoriesApi } from "@/lib/api/products";
import { tServer } from "@/lib/i18n/server";

export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("shop.index.metadata_title") };
}

// Where "View all categories" lands. Lists the live categories, so one made in
// the console appears here without a code change.
export default async function ShopIndexPage() {
  const t = await tServer();
  const categories = await listCategoriesApi();
  const count = (n: number) =>
    t(n === 1 ? "shop.index.count_one" : "shop.index.count_other", { n });

  const tiles = [
    { href: "/shop/all", label: t("plp.all_name"), sub: null as string | null },
    ...categories.map((c) => ({ href: `/shop/${c.slug}`, label: c.name, sub: count(c.productCount) })),
  ];

  return (
    <section className="container-hf" style={{ paddingBlock: "40px 88px" }}>
      <nav
        style={{
          display: "flex",
          gap: 8,
          fontSize: 12,
          letterSpacing: 0.04,
          color: "var(--fg-muted)",
          marginBottom: 16,
        }}
      >
        <Link href="/">{t("plp.crumb_root")}</Link>
        <span>·</span>
        <span style={{ color: "var(--fg-secondary)" }}>{t("shop.index.metadata_title")}</span>
      </nav>
      <div className="eyebrow">{t("shop.index.eyebrow")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(36px, 5vw, 56px)",
          lineHeight: 1.05,
          letterSpacing: "-0.015em",
          marginTop: 14,
          marginBottom: 40,
        }}
      >
        {t("shop.index.title")}
      </h1>

      {categories.length === 0 && (
        <p style={{ color: "var(--fg-secondary)", marginBottom: 24 }}>{t("shop.index.empty")}</p>
      )}

      <div className="cat-rail">
        {tiles.map((c) => (
          <Link key={c.href} href={c.href} className="cat-tile" style={{ aspectRatio: "4 / 3" }}>
            {c.sub && <span className="sublabel">{c.sub}</span>}
            <span className="label">{c.label}</span>
          </Link>
        ))}
      </div>
    </section>
  );
}
