"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { apiFetch } from "@/lib/api/client";
import type { Order } from "@/lib/api/orders";
import { formatPrice } from "@/lib/format";
import { useT } from "@/lib/i18n/context";
import { WhatsAppCta } from "@/components/whatsapp-cta";

export function ConfirmationClient() {
  const { t } = useT();
  const params = useSearchParams();
  const orderNum = params.get("order") ?? "";

  const [order, setOrder] = useState<Order | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!orderNum) {
      setErr(t("confirmation.missing_ref"));
      return;
    }
    let cancelled = false;
    apiFetch<Order>(`/orders/by-number/${encodeURIComponent(orderNum)}`)
      .then((o) => {
        if (!cancelled) setOrder(o);
      })
      .catch((e: unknown) => {
        const msg =
          e && typeof e === "object" && "message" in e
            ? String((e as { message?: unknown }).message)
            : t("confirmation.load_error");
        if (!cancelled) setErr(msg);
      });
    return () => {
      cancelled = true;
    };
  }, [orderNum, t]);

  return (
    <main>
      <section className="confirm-hero">
        <div className="container-hf" style={{ maxWidth: 680 }}>
          <Image
            src="/brand/logo-mark-navy.png"
            alt={t("common.brand")}
            width={80}
            height={80}
            style={{ borderRadius: "50%", margin: "0 auto 24px", display: "block" }}
          />
          <div className="eyebrow" style={{ color: "var(--hf-gold-300)" }}>
            {t("confirmation.eyebrow")}
          </div>
          <h1>{t("confirmation.title")}</h1>
          <p
            style={{
              fontFamily: "var(--font-serif)",
              fontStyle: "italic",
              fontSize: 20,
              color: "var(--hf-gold-300)",
              marginTop: 20,
            }}
          >
            {orderNum || t("confirmation.subline_no_order")}
            {order ? ` · ${formatPrice(order.total_amount, order.currency)}` : ""}
          </p>
          <p
            style={{
              fontSize: 15,
              lineHeight: 1.6,
              color: "rgba(245,239,224,.78)",
              maxWidth: 540,
              margin: "18px auto 0",
            }}
          >
            {t("confirmation.body")}
          </p>

          {/* ── Cash on delivery ─────────────────────────────────────────────
              While online payment is switched off, this panel carries the whole
              payment story: how much, to whom, and when. The MoMo USSD and
              hosted-checkout panels that used to live here are preserved in git
              history and return with the payment selector in checkout step 3. */}
          <div className="payment-instructions cod-confirm">
            <div className="eyebrow" style={{ color: "var(--hf-gold-300)" }}>
              {t("confirmation.cod_eyebrow")}
            </div>
            {/* Render the figure only once it is real. A placeholder dash set in
                40px display type reads as "the amount is nothing", which is the
                worst possible thing to say about a bill due on delivery. */}
            {order ? (
              <div className="cod-confirm-amount">
                {formatPrice(order.total_amount, order.currency)}
              </div>
            ) : (
              <div className="cod-confirm-amount-pending" aria-hidden />
            )}
            <p className="cod-confirm-body">{t("confirmation.cod_body")}</p>
            <ul className="cod-confirm-list">
              <li>{t("confirmation.cod_point_1")}</li>
              <li>{t("confirmation.cod_point_2")}</li>
            </ul>
          </div>

          <WhatsAppCta
            tone="dark"
            orderNumber={orderNum || undefined}
            amount={order ? formatPrice(order.total_amount, order.currency) : undefined}
          />

          <div
            style={{
              display: "flex",
              gap: 14,
              justifyContent: "center",
              marginTop: 36,
              flexWrap: "wrap",
            }}
          >
            <Link
              href="/"
              className="btn btn-secondary btn-lg"
              style={{ color: "var(--hf-cream-50)", borderColor: "var(--hf-cream-50)" }}
            >
              {t("confirmation.keep_browsing")}
            </Link>
            <Link
              href="/account"
              className="btn btn-secondary btn-lg"
              style={{ color: "var(--hf-cream-50)", borderColor: "var(--hf-cream-50)" }}
            >
              {t("confirmation.track")}
            </Link>
          </div>
        </div>
      </section>

      {order && (
        <section className="container-hf" style={{ padding: "48px 0 80px", maxWidth: 720 }}>
          <h2
            style={{
              fontFamily: "var(--font-display)",
              fontWeight: 500,
              fontSize: 26,
              marginBottom: 18,
            }}
          >
            {t("confirmation.receipt_title")}
          </h2>
          <div
            style={{
              background: "#fff",
              border: "1px solid var(--border-hair)",
              borderRadius: 12,
              padding: 24,
              display: "flex",
              flexDirection: "column",
              gap: 14,
            }}
          >
            {order.items.map((it) => (
              <div key={it.id} style={{ display: "flex", justifyContent: "space-between", gap: 12 }}>
                <div>
                  <div style={{ fontFamily: "var(--font-display)", fontSize: 16, fontWeight: 500 }}>
                    {it.product_name}
                  </div>
                  <div style={{ fontSize: 12, color: "var(--fg-muted)" }}>
                    EU {it.size_eu} · {it.color} · ×{it.quantity}
                  </div>
                </div>
                <div className="t-price" style={{ fontSize: 16 }}>
                  {formatPrice(it.subtotal, order.currency)}
                </div>
              </div>
            ))}
            <div className="divider" />
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 14 }}>
              <span>{t("bag.subtotal")}</span>
              <span>{formatPrice(order.subtotal, order.currency)}</span>
            </div>
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 14, color: "var(--fg-secondary)" }}>
              <span>{t("bag.courier")} ({order.delivery_method})</span>
              <span>
                {order.delivery_fee === 0
                  ? t("common.complimentary")
                  : formatPrice(order.delivery_fee, order.currency)}
              </span>
            </div>
            <div className="divider" />
            <div style={{ display: "flex", justifyContent: "space-between", fontFamily: "var(--font-display)", fontSize: 22, fontWeight: 500 }}>
              <span>{t("bag.total")}</span>
              <span>{formatPrice(order.total_amount, order.currency)}</span>
            </div>
          </div>
        </section>
      )}

      {err && (
        <section className="container-hf" style={{ padding: "32px 0" }}>
          <p style={{ color: "var(--fg-muted)", fontSize: 13 }}>{err}</p>
        </section>
      )}
    </main>
  );
}
