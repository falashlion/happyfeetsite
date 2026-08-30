import Link from "next/link";
import { tServer } from "@/lib/i18n/server";

export default async function NotFound() {
  const t = await tServer();
  return (
    <section className="container-hf" style={{ padding: "96px 0", textAlign: "center" }}>
      <div className="eyebrow">{t("notfound.eyebrow")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(40px,5vw,60px)",
          marginTop: 14,
          lineHeight: 1.05,
        }}
      >
        {t("notfound.title")}
      </h1>
      <p
        style={{
          fontFamily: "var(--font-serif)",
          fontStyle: "italic",
          fontSize: 20,
          color: "var(--fg-secondary)",
          marginTop: 18,
        }}
      >
        {t("notfound.body")}
      </p>
      <Link href="/" className="btn btn-primary btn-lg" style={{ marginTop: 32 }}>
        {t("notfound.cta")}
      </Link>
    </section>
  );
}
