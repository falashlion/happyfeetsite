import type { Metadata } from "next";
import Link from "next/link";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("about.metadata_title") };
}
export const dynamic = "force-dynamic";

export default async function AboutPage() {
  const t = await tServer();
  return (
    <section className="container-hf" style={{ padding: "72px 0", maxWidth: 720 }}>
      <div className="eyebrow">{t("about.eyebrow")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(36px,5vw,56px)",
          lineHeight: 1.06,
          letterSpacing: "-0.015em",
          marginTop: 14,
        }}
      >
        {t("about.title")}
      </h1>
      <div className="gold-rule" style={{ marginTop: 18 }} />
      <p
        style={{
          fontFamily: "var(--font-serif)",
          fontStyle: "italic",
          fontSize: 22,
          color: "var(--fg-secondary)",
          marginTop: 24,
          lineHeight: 1.5,
        }}
      >
        {t("about.lede")}
      </p>
      <div style={{ fontSize: 15, lineHeight: 1.7, color: "var(--fg-secondary)", marginTop: 28 }}>
        <p>{t("about.p1")}</p>
        <p style={{ marginTop: 16 }}>{t("about.p2")}</p>
      </div>
      <Link href="/shop/sneakers" className="btn btn-primary btn-lg" style={{ marginTop: 36 }}>
        {t("about.cta")}
      </Link>
    </section>
  );
}
