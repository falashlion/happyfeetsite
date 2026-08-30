"use client";

import { useState } from "react";
import { Sparkles, ChevronRight, Check } from "lucide-react";
import { applyCoupon, HF_COUPONS, type AppliedCoupon } from "@/lib/coupons";
import { useT } from "@/lib/i18n/context";

export function CouponBlock({
  subtotal,
  isMember,
  applied,
  onApply,
  onClear,
  compact,
}: {
  subtotal: number;
  isMember?: boolean;
  applied: AppliedCoupon | null;
  onApply: (a: AppliedCoupon) => void;
  onClear: () => void;
  compact?: boolean;
}) {
  const { t } = useT();
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  function submit() {
    const r = applyCoupon(code, { subtotal, isMember });
    if (!r.ok) {
      setError(t(r.errorKey, r.errorVars));
      return;
    }
    setError(null);
    setCode("");
    onApply({ coupon: r.coupon, discount: r.discount, shippingFree: r.shippingFree });
  }

  if (applied) {
    return (
      <div className="coupon-applied">
        <span className="name">
          <Check size={14} style={{ stroke: "var(--hf-gold-700)" }} />
          {t(`coupon.${applied.coupon.code}.label`)}
          <span style={{ color: "var(--hf-gold-700)", marginLeft: 6, opacity: 0.6, fontFamily: "var(--font-mono)" }}>
            · {applied.coupon.code}
          </span>
        </span>
        <button onClick={onClear}>{t("coupon.remove")}</button>
      </div>
    );
  }

  return (
    <div>
      {/* Not a <form>: this block renders inside the checkout's order form, and
          a nested form is invalid HTML — React flags it as a hydration error and
          the browser flattens it, so Enter here would submit the *order*.
          Enter is handled explicitly on the input instead. */}
      <div className="coupon-card">
        <Sparkles size={16} style={{ stroke: "var(--hf-gold-600)", flexShrink: 0 }} />
        <input
          className="coupon-input"
          placeholder={t("coupon.placeholder")}
          value={code}
          onChange={(e) => {
            setCode(e.target.value);
            setError(null);
          }}
          onKeyDown={(e) => {
            if (e.key !== "Enter") return;
            e.preventDefault(); // never let Enter reach the enclosing order form
            if (code.trim()) submit();
          }}
        />
        <button
          type="button"
          className="btn btn-secondary btn-sm"
          onClick={submit}
          disabled={!code.trim()}
        >
          {t("coupon.apply")}
        </button>
      </div>
      {error && (
        <p style={{ fontSize: 12, color: "var(--hf-danger)", marginTop: 8, paddingLeft: 4 }}>
          {error}
        </p>
      )}
      {!compact && (
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          style={{
            marginTop: 10,
            padding: "4px 0",
            background: "transparent",
            border: "none",
            fontFamily: "var(--font-body)",
            fontSize: 12,
            color: "var(--fg-muted)",
            cursor: "pointer",
            letterSpacing: "0.04em",
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
          }}
        >
          {open ? "−" : "+"} {t("coupon.try_ours")}
        </button>
      )}
      {!compact && open && (
        <div style={{ marginTop: 10, display: "flex", flexDirection: "column", gap: 6 }}>
          {Object.values(HF_COUPONS).map((c) => (
            <button
              type="button"
              key={c.code}
              onClick={() => setCode(c.code)}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 12,
                padding: "8px 10px",
                background: "var(--hf-cream-50)",
                border: "1px solid var(--border-hair)",
                borderRadius: 6,
                cursor: "pointer",
                fontFamily: "var(--font-body)",
                textAlign: "left",
              }}
            >
              <span style={{ display: "inline-flex", flexDirection: "column", gap: 2 }}>
                <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, fontWeight: 600, letterSpacing: "0.06em" }}>
                  {c.code}
                </span>
                <span style={{ fontSize: 11, color: "var(--fg-muted)" }}>
                  {t(`coupon.${c.code}.description`)}
                </span>
              </span>
              <ChevronRight size={14} style={{ stroke: "var(--fg-muted)" }} />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
