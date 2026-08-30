"use client";

import { Heart, Check } from "lucide-react";
import { useOptimistic, useState, useTransition } from "react";
import type { Product } from "@/lib/types";
import { useCart } from "@/store/cart";
import { useT } from "@/lib/i18n/context";

export function PdpPicker({ product }: { product: Product }) {
  const { t } = useT();
  const [sizeIdx, setSizeIdx] = useState<number | null>(null);
  const [colorIdx, setColorIdx] = useState(0);
  const [isPending, startTransition] = useTransition();
  const add = useCart((s) => s.add);

  const [optimisticAdded, setOptimisticAdded] = useOptimistic(false);

  const usSize = (eu: number) => (eu - 33).toFixed(0);
  const ukSize = (eu: number) => (eu - 34).toFixed(0);

  const handleAdd = () => {
    if (sizeIdx == null) return;
    const size = product.sizes[sizeIdx];
    const color = product.colors[colorIdx];
    const sku = product.skus?.find(
      (s) => s.sizeEu === size && s.color.toLowerCase() === color.name.toLowerCase(),
    );
    startTransition(() => {
      setOptimisticAdded(true);
      add({
        productId: product.id,
        skuId: sku?.id,
        size,
        color,
        qty: 1,
      });
      setTimeout(() => {
        // useOptimistic resets on its own after the transition; this keeps the
        // "Added" state visible for ~2s without sticky local state.
      }, 2000);
    });
  };

  const showAdded = optimisticAdded || isPending;

  return (
    <>
      <div>
        <div className="picker-label">
          <span>{t("pdp.picker.color")}</span>
          <span className="chosen">{product.colors[colorIdx].name}</span>
        </div>
        <div className="color-row">
          {product.colors.map((c, i) => (
            <button
              key={c.name}
              className={`color-swatch ${i === colorIdx ? "active" : ""}`}
              style={{
                background: c.hex,
                borderColor: c.hex === "#F5EFE0" ? "var(--border-soft)" : "transparent",
              }}
              onClick={() => setColorIdx(i)}
              aria-label={c.name}
              aria-pressed={i === colorIdx}
            />
          ))}
        </div>
      </div>

      <div>
        <div className="picker-label">
          <span>{t("pdp.picker.size_eu")}</span>
          <a
            href="#sizing-guide"
            style={{
              fontSize: 11,
              letterSpacing: ".04em",
              textTransform: "none",
              color: "var(--fg-link)",
              cursor: "pointer",
              textDecoration: "underline",
            }}
          >
            {t("pdp.picker.sizing_guide")}
          </a>
        </div>
        <div className="size-grid">
          {product.sizes.map((s, i) => {
            const oos = product.oosSizes.includes(s);
            return (
              <button
                key={s}
                type="button"
                className={`size-cell ${sizeIdx === i ? "active" : ""} ${oos ? "oos" : ""}`}
                disabled={oos}
                onClick={() => !oos && setSizeIdx(i)}
                aria-pressed={sizeIdx === i}
              >
                {s}
              </button>
            );
          })}
        </div>
        {sizeIdx != null && (
          <div
            style={{
              fontSize: 12,
              color: "var(--fg-muted)",
              marginTop: 10,
              fontFamily: "var(--font-mono)",
            }}
          >
            {t("pdp.picker.size_format", {
              eu: product.sizes[sizeIdx],
              us: usSize(product.sizes[sizeIdx]),
              uk: ukSize(product.sizes[sizeIdx]),
            })}
          </div>
        )}
      </div>

      <div style={{ display: "flex", gap: 10, marginTop: 12 }}>
        <button
          className="btn btn-primary btn-lg btn-block"
          disabled={sizeIdx == null}
          onClick={handleAdd}
        >
          {showAdded ? (
            <>
              <Check size={18} strokeWidth={1.5} /> {t("pdp.cta.added")}
            </>
          ) : sizeIdx == null ? (
            t("pdp.cta.select_size")
          ) : (
            t("pdp.cta.add")
          )}
        </button>
        <button
          type="button"
          className="btn btn-secondary btn-lg"
          aria-label={t("pdp.save_aria")}
          style={{ flexShrink: 0, padding: "16px" }}
        >
          <Heart size={18} strokeWidth={1.5} />
        </button>
      </div>
    </>
  );
}
