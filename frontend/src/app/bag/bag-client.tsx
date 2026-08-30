"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { LazyMotion, domAnimation, m, AnimatePresence } from "framer-motion";
import { Minus, Plus, ArrowRight, Sparkles } from "lucide-react";
import { useCart } from "@/store/cart";
import { useCoupon } from "@/store/coupon";
import { HF_PRODUCTS } from "@/lib/catalog";
import { formatPrice } from "@/lib/format";
import { computeTotals } from "@/lib/coupons";
import { getProductByIdApi } from "@/lib/api/products";
import { CouponBlock } from "@/components/coupon-block";
import { useT } from "@/lib/i18n/context";
import type { Product } from "@/lib/types";

const SHIPPING_FREE_THRESHOLD = 50000;
const BASE_SHIPPING = 2000;

export function BagClient({ blurs }: { blurs: Record<string, string> }) {
  const { t } = useT();
  const items = useCart((s) => s.items);
  const hydrated = useCart((s) => s.hydrated);
  const remove = useCart((s) => s.remove);
  const setQty = useCart((s) => s.setQty);
  const applied = useCoupon((s) => s.applied);
  const setCoupon = useCoupon((s) => s.set);

  // Resolve products: try the local static catalog first (instant), then fall
  // back to the API for ids that aren't in the catalog (e.g. backend UUIDs).
  const [extraProducts, setExtraProducts] = useState<Record<string, Product>>({});

  useEffect(() => {
    const missing = items
      .map((it) => it.productId)
      .filter((id) => !HF_PRODUCTS.find((p) => p.id === id) && !extraProducts[id]);
    if (missing.length === 0) return;
    let cancelled = false;
    Promise.all(missing.map((id) => getProductByIdApi(id))).then((results) => {
      if (cancelled) return;
      const next: Record<string, Product> = {};
      results.forEach((p, i) => {
        if (p) next[missing[i]] = p;
      });
      if (Object.keys(next).length > 0) {
        setExtraProducts((prev) => ({ ...prev, ...next }));
      }
    });
    return () => {
      cancelled = true;
    };
  }, [items, extraProducts]);

  const enriched = useMemo(
    () =>
      items.flatMap((it) => {
        const product =
          HF_PRODUCTS.find((p) => p.id === it.productId) ?? extraProducts[it.productId];
        return product ? [{ ...it, product }] : [];
      }),
    [items, extraProducts],
  );

  if (!hydrated) return <BagSkeleton />;
  if (enriched.length === 0) return <EmptyState t={t} />;

  const subtotal = enriched.reduce((s, i) => s + i.product.price * i.qty, 0);
  const baseShipping = subtotal >= SHIPPING_FREE_THRESHOLD ? 0 : BASE_SHIPPING;
  const totals = computeTotals({ subtotal, deliveryFee: baseShipping, applied });
  const { discount, shipping, total } = totals;
  const toFree = Math.max(0, SHIPPING_FREE_THRESHOLD - subtotal);

  return (
    <LazyMotion features={domAnimation} strict>
      <section className="container-hf" style={{ padding: "40px 0 0" }}>
        <div className="eyebrow">{t("bag.eyebrow")}</div>
        <h1
          style={{
            fontFamily: "var(--font-display)",
            fontWeight: 500,
            fontSize: "clamp(32px,4vw,44px)",
            marginTop: 14,
            lineHeight: 1.1,
          }}
        >
          {t(enriched.length === 1 ? "bag.reserved_one" : "bag.reserved_other", { n: enriched.length })}
        </h1>
        <div className="gold-rule" style={{ marginTop: 14 }} />
      </section>

      <section
        className="container-hf checkout-grid"
        style={{ padding: "40px 0 80px" }}
      >
        <div>
          <AnimatePresence initial={false}>
            {enriched.map((it, idx) => (
              <m.div
                key={`${it.productId}-${it.size}-${it.color.name}`}
                layout
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, x: -32 }}
                transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
                className="bag-row"
              >
                <div className="bag-thumb">
                  <Image
                    src={it.product.image}
                    alt={`${it.product.brand} ${it.product.name}`}
                    fill
                    sizes="100px"
                    placeholder={blurs[it.product.image] ? "blur" : "empty"}
                    blurDataURL={blurs[it.product.image]}
                  />
                </div>
                <div className="bag-meta">
                  <div className="brand">{it.product.brand}</div>
                  <div className="name">{it.product.name}</div>
                  <div className="var">
                    {it.color.name} · {t("bag.size")} EU {it.size}
                  </div>
                  <div className="qty-stepper" role="group" aria-label={t("bag.size")}>
                    <button
                      onClick={() => setQty(idx, Math.max(1, it.qty - 1))}
                      aria-label={t("bag.qty_decrease")}
                    >
                      <Minus size={14} strokeWidth={1.5} />
                    </button>
                    <span className="n">{it.qty}</span>
                    <button
                      onClick={() => setQty(idx, it.qty + 1)}
                      aria-label={t("bag.qty_increase")}
                    >
                      <Plus size={14} strokeWidth={1.5} />
                    </button>
                  </div>
                </div>
                <div style={{ textAlign: "right" }}>
                  <div className="t-price" style={{ fontSize: 18 }}>
                    {formatPrice(it.product.price * it.qty, it.product.currency)}
                  </div>
                  <button
                    onClick={() => remove(idx)}
                    style={{
                      background: "none",
                      border: "none",
                      color: "var(--fg-muted)",
                      fontSize: 12,
                      marginTop: 12,
                      cursor: "pointer",
                      letterSpacing: ".06em",
                    }}
                  >
                    {t("bag.remove")}
                  </button>
                </div>
              </m.div>
            ))}
          </AnimatePresence>
        </div>

        <aside>
          <div className="summary">
            <div className="eyebrow" style={{ marginBottom: 4 }}>
              {t("bag.summary")}
            </div>
            <div className="row">
              <span>{t("bag.subtotal")}</span>
              <span>{formatPrice(subtotal)}</span>
            </div>
            {discount > 0 && applied?.coupon && (
              <div className="discount-line">
                <span className="name">
                  <Sparkles size={12} style={{ stroke: "var(--hf-gold-700)" }} />
                  {applied.coupon.code}
                </span>
                <span>− {formatPrice(discount)}</span>
              </div>
            )}
            <div className="row muted">
              <span>{t("bag.courier")}</span>
              <span>{shipping === 0 ? t("common.complimentary") : formatPrice(shipping)}</span>
            </div>
            {toFree > 0 && shipping > 0 && (
              <div
                style={{
                  fontSize: 12,
                  color: "var(--fg-muted)",
                  fontStyle: "italic",
                  fontFamily: "var(--font-serif)",
                }}
              >
                {t("bag.add_more", { amount: formatPrice(toFree) })}
              </div>
            )}

            <CouponBlock
              subtotal={subtotal}
              applied={applied}
              onApply={setCoupon}
              onClear={() => setCoupon(null)}
            />

            <div className="divider" />
            <div className="row total">
              <span>{t("bag.total")}</span>
              <span>{formatPrice(total)}</span>
            </div>
            <Link
              href="/checkout"
              className="btn btn-primary btn-lg btn-block"
              style={{ marginTop: 12 }}
            >
              {t("bag.continue")} <ArrowRight size={16} strokeWidth={1.5} />
            </Link>
            <p
              style={{
                fontSize: 11,
                color: "var(--fg-muted)",
                textAlign: "center",
                marginTop: 8,
                letterSpacing: ".06em",
              }}
            >
              {t("bag.payment_line")}
            </p>
          </div>
        </aside>
      </section>
    </LazyMotion>
  );
}

function EmptyState({ t }: { t: (k: string, v?: Record<string, string | number>) => string }) {
  return (
    <section
      className="container-hf"
      style={{ padding: "80px 0", textAlign: "center", maxWidth: 560 }}
    >
      <div className="eyebrow">{t("bag.eyebrow")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(36px,5vw,48px)",
          marginTop: 14,
          lineHeight: 1.1,
        }}
      >
        {t("bag.empty_title")}
      </h1>
      <p
        style={{
          fontFamily: "var(--font-serif)",
          fontStyle: "italic",
          fontSize: 19,
          color: "var(--fg-secondary)",
          marginTop: 18,
        }}
      >
        {t("bag.empty_lede")}
      </p>
      <Link href="/shop/sneakers" className="btn btn-primary btn-lg" style={{ marginTop: 28 }}>
        {t("bag.empty_cta")}
      </Link>
    </section>
  );
}

function BagSkeleton() {
  return (
    <section className="container-hf" style={{ padding: "80px 0" }}>
      <div
        style={{
          height: 240,
          background: "var(--bg-sunken)",
          borderRadius: 12,
          opacity: 0.4,
        }}
      />
    </section>
  );
}
