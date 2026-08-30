import { notFound } from "next/navigation";
import Link from "next/link";
import type { Metadata } from "next";
import { Truck, RotateCcw, Shield } from "lucide-react";
import { HF_PRODUCTS, HF_CATEGORIES } from "@/lib/catalog";
import { getProductBySlugApi } from "@/lib/api/products";
import { getBlurMap } from "@/lib/placeholder";
import { formatPrice } from "@/lib/format";
import { Rating } from "@/components/rating";
import { PdpPicker } from "./pdp-picker";
import { PdpGalleryClient } from "./pdp-gallery";
import { tServer } from "@/lib/i18n/server";

export const dynamic = "force-dynamic";

export function generateStaticParams() {
  return HF_PRODUCTS.map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const product = await getProductBySlugApi(slug);
  if (!product) return { title: "Not found" };
  return {
    title: `${product.name} — ${product.brand}`,
    description: product.subtitle,
  };
}

export default async function ProductPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const t = await tServer();
  const product = await getProductBySlugApi(slug);
  if (!product) notFound();

  const blurs = await getBlurMap(product.images);
  const category =
    HF_CATEGORIES.find((c) => c.slug === product.category)?.name ?? product.category;

  return (
    <section className="container-hf">
      <nav
        style={{
          display: "flex",
          gap: 8,
          fontSize: 12,
          letterSpacing: 0.04,
          color: "var(--fg-muted)",
          padding: "28px 0 0",
        }}
      >
        <Link href="/">{t("plp.crumb_root")}</Link>
        <span>·</span>
        <Link
          href={`/shop/${product.category}`}
          style={{ color: "var(--fg-secondary)" }}
        >
          {category}
        </Link>
        <span>·</span>
        <span style={{ color: "var(--fg-secondary)" }}>{product.name}</span>
      </nav>

      <div className="pdp">
        <PdpGalleryClient
          images={product.images}
          blurs={blurs}
          altBase={`${product.brand} ${product.name}`}
        />

        <div className="pdp-info">
          <div>
            <div className="brand-eyebrow">{product.brand}</div>
            <h1 style={{ marginTop: 8 }}>{product.name}</h1>
            <p
              style={{
                fontFamily: "var(--font-serif)",
                fontStyle: "italic",
                fontSize: 18,
                color: "var(--fg-secondary)",
                marginTop: 8,
              }}
            >
              {product.subtitle}
            </p>
          </div>

          <Rating value={product.rating} count={product.ratingCount} />

          <div className="price">
            {formatPrice(product.price, product.currency)}
            {product.was && (
              <>
                <span className="was">{formatPrice(product.was, product.currency)}</span>
                <span className="discount-badge">
                  −{Math.round((1 - product.price / product.was) * 100)}%
                </span>
              </>
            )}
          </div>

          <p className="desc">{product.description}</p>

          <PdpPicker product={product} />

          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 14,
              marginTop: 8,
              paddingTop: 24,
              borderTop: "1px solid var(--border-hair)",
            }}
          >
            {[
              { Icon: Truck, body: t("pdp.promise.courier") },
              { Icon: RotateCcw, body: t("pdp.promise.returns") },
              { Icon: Shield, body: t("pdp.promise.authentic") },
            ].map(({ Icon, body }) => (
              <div key={body} style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <Icon size={18} strokeWidth={1.5} style={{ color: "var(--hf-gold-600)" }} />
                <span style={{ fontSize: 13, color: "var(--fg-secondary)" }}>{body}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <section
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: 48,
          padding: "48px 0 96px",
          borderTop: "1px solid var(--border-hair)",
        }}
      >
        <div>
          <div className="eyebrow">{t("pdp.materials_eyebrow")}</div>
          <h3
            style={{
              fontFamily: "var(--font-display)",
              fontSize: 24,
              fontWeight: 500,
              margin: "10px 0 18px",
            }}
          >
            {t("pdp.materials_title")}
          </h3>
          <ul style={{ listStyle: "none", display: "flex", flexDirection: "column", gap: 10 }}>
            {product.materials.map((m) => (
              <li
                key={m}
                style={{
                  display: "flex",
                  gap: 10,
                  fontSize: 14,
                  color: "var(--fg-secondary)",
                  lineHeight: 1.5,
                }}
              >
                <span style={{ color: "var(--hf-gold-600)" }}>—</span>
                {m}
              </li>
            ))}
          </ul>
        </div>
        <div>
          <div className="eyebrow">{t("pdp.reviews_eyebrow")}</div>
          <h3
            style={{
              fontFamily: "var(--font-display)",
              fontSize: 24,
              fontWeight: 500,
              margin: "10px 0 18px",
            }}
          >
            {t("pdp.reviews_count", {
              count: product.ratingCount,
              rating: product.rating.toFixed(1),
            })}
          </h3>
          <div
            style={{
              background: "#fff",
              border: "1px solid var(--border-hair)",
              borderRadius: 8,
              padding: 20,
            }}
          >
            <Rating value={5} />
            <p
              style={{
                fontFamily: "var(--font-serif)",
                fontStyle: "italic",
                fontSize: 17,
                marginTop: 12,
                color: "var(--fg-primary)",
                lineHeight: 1.5,
              }}
            >
              {t("pdp.review_quote")}
            </p>
            <div
              style={{
                fontSize: 12,
                color: "var(--fg-muted)",
                marginTop: 10,
                letterSpacing: ".04em",
              }}
            >
              {t("pdp.review_meta")}
            </div>
          </div>
        </div>
      </section>
    </section>
  );
}
