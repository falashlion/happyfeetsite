"use client";

import Link from "next/link";
import { useMemo, useState, useTransition } from "react";
import type { Product } from "@/lib/types";
import { ProductCard } from "@/components/product-card";
import { useT } from "@/lib/i18n/context";

type Sort = "relevance" | "price_asc" | "price_desc" | "rating";

export function PlpClient({
  categorySlug,
  categoryName,
  title,
  eyebrow,
  brands,
  initial,
  blurs,
}: {
  categorySlug: string;
  categoryName: string;
  title: string;
  eyebrow: string;
  brands: string[];
  initial: Product[];
  blurs: Record<string, string>;
}) {
  const { t } = useT();
  const [sort, setSort] = useState<Sort>("relevance");
  const [brand, setBrand] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();

  const products = useMemo(() => {
    let list = initial;
    if (brand) list = list.filter((p) => p.brand === brand);
    const sorted = [...list];
    switch (sort) {
      case "price_asc":
        sorted.sort((a, b) => a.price - b.price);
        break;
      case "price_desc":
        sorted.sort((a, b) => b.price - a.price);
        break;
      case "rating":
        sorted.sort((a, b) => b.rating - a.rating);
        break;
    }
    return sorted;
  }, [initial, brand, sort]);

  const noun = products.length === 1 ? t("common.pair_one") : t("common.pair_other");

  void categorySlug; // reserved for future analytics

  return (
    <>
      <section className="container-hf" style={{ padding: "40px 0 24px" }}>
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
          <span style={{ color: "var(--fg-secondary)" }}>{categoryName}</span>
        </nav>
        <div className="eyebrow">{eyebrow}</div>
        <h1
          style={{
            fontFamily: "var(--font-display)",
            fontWeight: 500,
            fontSize: "clamp(36px, 5vw, 56px)",
            lineHeight: 1.05,
            letterSpacing: "-0.015em",
            marginTop: 14,
          }}
        >
          {title}
        </h1>
        <div className="gold-rule" style={{ marginTop: 18 }} />
      </section>

      <section className="container-hf" style={{ paddingBottom: 32 }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            gap: 24,
            flexWrap: "wrap",
            padding: "18px 0",
            borderTop: "1px solid var(--border-hair)",
            borderBottom: "1px solid var(--border-hair)",
          }}
        >
          <div
            style={{
              display: "flex",
              gap: 10,
              alignItems: "center",
              overflowX: "auto",
              flex: 1,
            }}
          >
            <span className="eyebrow eyebrow-muted" style={{ flexShrink: 0 }}>
              {t("plp.brand")}
            </span>
            <ChipButton active={brand === null} onClick={() => startTransition(() => setBrand(null))}>
              {t("plp.brand_all")}
            </ChipButton>
            {brands.map((b) => (
              <ChipButton
                key={b}
                active={brand === b}
                onClick={() => startTransition(() => setBrand(b))}
              >
                {b}
              </ChipButton>
            ))}
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span className="eyebrow eyebrow-muted">{t("plp.sort")}</span>
            <select
              value={sort}
              onChange={(e) =>
                startTransition(() => setSort(e.target.value as Sort))
              }
              style={{
                fontFamily: "var(--font-body)",
                fontSize: 13,
                padding: "8px 12px",
                background: "#fff",
                border: "1px solid var(--border-soft)",
                borderRadius: 4,
                outline: "none",
              }}
            >
              <option value="relevance">{t("plp.sort.relevance")}</option>
              <option value="price_asc">{t("plp.sort.price_asc")}</option>
              <option value="price_desc">{t("plp.sort.price_desc")}</option>
              <option value="rating">{t("plp.sort.rating")}</option>
            </select>
          </div>
        </div>
      </section>

      <section className="container-hf">
        <p style={{ fontSize: 13, color: "var(--fg-secondary)", marginBottom: 24 }}>
          {t("plp.count", { n: products.length, noun })}
          {brand && (
            <>
              {" "}{t("plp.count.in")}{" "}
              <em style={{ fontStyle: "normal", color: "var(--fg-primary)" }}>{brand}</em>
            </>
          )}
        </p>
        {products.length === 0 ? (
          <div style={{ padding: "80px 0", textAlign: "center" }}>
            <p
              style={{
                fontFamily: "var(--font-serif)",
                fontStyle: "italic",
                fontSize: 22,
                color: "var(--fg-secondary)",
              }}
            >
              {t("plp.empty")}
            </p>
          </div>
        ) : (
          <div
            className="product-grid"
            style={{
              paddingBottom: 64,
              opacity: isPending ? 0.6 : 1,
              transition: "opacity 240ms cubic-bezier(0.16,1,0.3,1)",
            }}
          >
            {products.map((p, i) => (
              <ProductCard
                key={p.id}
                product={p}
                priority={i < 4}
                blurDataURL={blurs[p.image]}
              />
            ))}
          </div>
        )}
      </section>
    </>
  );
}

function ChipButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      style={{
        fontFamily: "var(--font-body)",
        fontSize: 13,
        fontWeight: 500,
        padding: "7px 14px",
        borderRadius: 999,
        background: active ? "var(--hf-ink-900)" : "#fff",
        color: active ? "var(--hf-cream-50)" : "var(--fg-primary)",
        border: `1px solid ${active ? "var(--hf-ink-900)" : "var(--border-soft)"}`,
        cursor: "pointer",
        whiteSpace: "nowrap",
        transition: "all 240ms cubic-bezier(0.16,1,0.3,1)",
      }}
    >
      {children}
    </button>
  );
}
