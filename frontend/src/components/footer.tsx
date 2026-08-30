import Image from "next/image";
import Link from "next/link";
import { Mail } from "lucide-react";
import { tServer } from "@/lib/i18n/server";

export async function Footer() {
  const t = await tServer();
  const COLS: { title: string; items: { label: string; href: string }[] }[] = [
    {
      title: t("footer.col_shop"),
      items: [
        { label: t("nav.women"), href: "/shop/womens" },
        { label: t("nav.men"), href: "/shop/mens" },
        { label: t("nav.kids"), href: "/shop/kids" },
        { label: t("nav.sneakers"), href: "/shop/sneakers" },
        { label: t("nav.boots"), href: "/shop/boots" },
        { label: t("nav.sale"), href: "/sale" },
      ],
    },
    {
      title: t("footer.col_house"),
      items: [
        { label: t("footer.house.story"), href: "/about" },
        { label: t("footer.house.materials"), href: "/about" },
        { label: t("footer.house.concierge"), href: "/about" },
        { label: t("footer.house.stores"), href: "/about" },
      ],
    },
    {
      title: t("footer.col_care"),
      items: [
        { label: t("footer.care.courier"), href: "/about" },
        { label: t("footer.care.returns"), href: "/about" },
        { label: t("footer.care.sizing"), href: "/about" },
        { label: t("footer.care.contact"), href: "/about" },
      ],
    },
  ];

  return (
    <footer style={{ background: "var(--hf-navy-900)", color: "var(--hf-cream-50)", marginTop: 96 }}>
      <div className="container-hf" style={{ padding: "72px clamp(20px,4vw,48px) 28px" }}>
        {/* Column layout lives in globals.css (.footer-grid) so it can respond
            to viewport width — an inline gridTemplateColumns would win over any
            media query and pin the footer to five columns on a phone. */}
        <div className="footer-grid">
          <div>
            <Image
              src="/brand/logo-mark-navy.png"
              alt={t("common.brand")}
              width={56}
              height={56}
              style={{ borderRadius: "50%" }}
            />
            <div style={{ fontFamily: "var(--font-display)", fontWeight: 600, fontSize: 24, marginTop: 14 }}>
              {t("common.brand")}
            </div>
            <div
              style={{
                fontFamily: "var(--font-serif)",
                fontStyle: "italic",
                fontSize: 15,
                color: "var(--hf-gold-300)",
                marginTop: 2,
              }}
            >
              {t("common.tagline")}
            </div>
            <p
              style={{
                fontSize: 13,
                lineHeight: 1.6,
                color: "rgba(245,239,224,.66)",
                marginTop: 18,
                maxWidth: 280,
              }}
            >
              {t("footer.about_blurb")}
            </p>
          </div>
          {COLS.map((col) => (
            <div key={col.title}>
              <div
                style={{
                  fontSize: 10,
                  letterSpacing: ".22em",
                  textTransform: "uppercase",
                  color: "var(--hf-gold-300)",
                  fontWeight: 600,
                  marginBottom: 14,
                }}
              >
                {col.title}
              </div>
              {col.items.map((it) => (
                <Link
                  key={it.label}
                  href={it.href}
                  style={{
                    display: "block",
                    fontSize: 13,
                    color: "rgba(245,239,224,.78)",
                    marginBottom: 9,
                  }}
                >
                  {it.label}
                </Link>
              ))}
            </div>
          ))}
          <div>
            <div
              style={{
                fontSize: 10,
                letterSpacing: ".22em",
                textTransform: "uppercase",
                color: "var(--hf-gold-300)",
                fontWeight: 600,
                marginBottom: 14,
              }}
            >
              {t("footer.col_newsletter")}
            </div>
            <p
              style={{
                fontSize: 13,
                lineHeight: 1.6,
                color: "rgba(245,239,224,.78)",
                marginBottom: 14,
              }}
            >
              {t("footer.newsletter_blurb")}
            </p>
            <form action="/api/newsletter" method="post" style={{ display: "flex", gap: 8 }}>
              <input
                placeholder={t("footer.newsletter_placeholder")}
                required
                type="email"
                name="email"
                style={{
                  flex: 1,
                  background: "transparent",
                  border: "1px solid var(--hf-gold-700)",
                  color: "var(--hf-cream-50)",
                  padding: "10px 12px",
                  fontSize: 13,
                  fontFamily: "var(--font-body)",
                  outline: "none",
                  borderRadius: 2,
                }}
              />
              <button type="submit" className="btn btn-gold btn-sm">
                <Mail size={14} strokeWidth={1.5} /> {t("footer.newsletter_cta")}
              </button>
            </form>
          </div>
        </div>
        <div
          style={{
            borderTop: "1px solid var(--hf-gold-700)",
            marginTop: 48,
            paddingTop: 18,
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            flexWrap: "wrap",
            gap: 14,
            fontSize: 11,
            color: "rgba(245,239,224,.55)",
            letterSpacing: ".06em",
          }}
        >
          <span>{t("footer.copyright")}</span>
          <span style={{ display: "inline-flex", gap: 18 }}>
            <Link href="/about">{t("footer.legal.privacy")}</Link>
            <Link href="/about">{t("footer.legal.terms")}</Link>
            <Link href="/about">{t("footer.legal.cookies")}</Link>
          </span>
        </div>
      </div>
    </footer>
  );
}
