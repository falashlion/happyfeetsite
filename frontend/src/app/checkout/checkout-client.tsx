"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Banknote } from "lucide-react";
import { useCart } from "@/store/cart";
import { useCoupon } from "@/store/coupon";
import { HF_PRODUCTS } from "@/lib/catalog";
import { formatPrice } from "@/lib/format";
import { computeTotals } from "@/lib/coupons";
import { CouponBlock } from "@/components/coupon-block";
import { isLoggedIn, me } from "@/lib/api/auth";
import { apiFetch, type ApiError } from "@/lib/api/client";
import { addCartItem, clearCart } from "@/lib/api/cart";
import { createOrder, type CreateOrderInput } from "@/lib/api/orders";
import { initiatePayment } from "@/lib/api/payments";
import { getProductByIdApi } from "@/lib/api/products";
import { useT } from "@/lib/i18n/context";
import type { Product } from "@/lib/types";

type Address = {
  id: string;
  label: string;
  recipient_name: string;
  recipient_phone: string | null;
  street: string;
  city: string;
  country: string;
  is_default: boolean;
};

type DeliveryMethod = CreateOrderInput["delivery_method"];
type PaymentMethod = CreateOrderInput["payment_method"];

const SHIPPING_FREE_THRESHOLD = 50000;

function readErrorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    const m = (err as ApiError).message;
    if (typeof m === "string" && m.length > 0) return m;
  }
  return "Something went wrong. Try again.";
}

export function CheckoutClient() {
  const router = useRouter();
  const { t } = useT();
  const items = useCart((s) => s.items);
  const hydrated = useCart((s) => s.hydrated);
  const clearLocalCart = useCart((s) => s.clear);
  const applied = useCoupon((s) => s.applied);
  const setCoupon = useCoupon((s) => s.set);

  const [authChecked, setAuthChecked] = useState(false);
  const [authed, setAuthed] = useState(false);

  const [addresses, setAddresses] = useState<Address[]>([]);
  const [selectedAddress, setSelectedAddress] = useState<string>("");
  const [addingAddress, setAddingAddress] = useState(false);
  const [newAddr, setNewAddr] = useState({
    label: "Home",
    recipient_name: "",
    recipient_phone: "",
    street: "",
    city: "Douala",
    country: "CM",
  });

  const [delivery, setDelivery] = useState<DeliveryMethod>("STANDARD");

  // ── Payment: cash on delivery only ────────────────────────────────────────
  // The online providers (MTN MoMo, Orange Money, Stripe) stay wired end to end
  // in the API — only the storefront selection is switched off, so re-enabling
  // them means restoring the `setPayment` state below and un-commenting the
  // radio group in step three. Until then every order is placed as COD.
  const payment: PaymentMethod = "cash_on_delivery";
  // const [payment, setPayment] = useState<PaymentMethod>("mtn_momo");
  // const [momoPhone, setMomoPhone] = useState("+237612345678");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Resolve extra products that came from the live API (UUID ids).
  const [extra, setExtra] = useState<Record<string, Product>>({});
  useEffect(() => {
    const missing = items
      .map((it) => it.productId)
      .filter((id) => !HF_PRODUCTS.find((p) => p.id === id) && !extra[id]);
    if (missing.length === 0) return;
    let cancelled = false;
    Promise.all(missing.map((id) => getProductByIdApi(id))).then((results) => {
      if (cancelled) return;
      const next: Record<string, Product> = {};
      results.forEach((p, i) => {
        if (p) next[missing[i]] = p;
      });
      if (Object.keys(next).length > 0) setExtra((p) => ({ ...p, ...next }));
    });
    return () => {
      cancelled = true;
    };
  }, [items, extra]);

  // Auth + initial address load
  useEffect(() => {
    let cancelled = false;
    async function init() {
      if (!isLoggedIn()) {
        if (!cancelled) {
          setAuthed(false);
          setAuthChecked(true);
        }
        return;
      }
      const u = await me();
      if (cancelled) return;
      if (!u) {
        setAuthed(false);
        setAuthChecked(true);
        return;
      }
      setAuthed(true);
      // Prefill new-address recipient with the user's name
      setNewAddr((s) => ({
        ...s,
        recipient_name: `${u.first_name} ${u.last_name}`.trim(),
        recipient_phone: u.phone ?? s.recipient_phone,
      }));
      try {
        const res = await apiFetch<{ data: Address[] }>("/users/me/addresses");
        if (cancelled) return;
        const list = res.data ?? [];
        setAddresses(list);
        const def = list.find((a) => a.is_default) ?? list[0];
        if (def) setSelectedAddress(def.id);
        else setAddingAddress(true);
      } catch {
        setAddingAddress(true);
      } finally {
        setAuthChecked(true);
      }
    }
    init();
    return () => {
      cancelled = true;
    };
  }, []);

  const enriched = useMemo(
    () =>
      items.flatMap((it) => {
        const product = HF_PRODUCTS.find((p) => p.id === it.productId) ?? extra[it.productId];
        return product ? [{ ...it, product }] : [];
      }),
    [items, extra],
  );

  const subtotal = enriched.reduce((s, i) => s + i.product.price * i.qty, 0);
  const deliveryFees: Record<DeliveryMethod, number> = {
    STANDARD: subtotal >= SHIPPING_FREE_THRESHOLD ? 0 : 2000,
    EXPRESS: 4000,
    SAME_DAY: 8000,
  };
  const totals = computeTotals({ subtotal, deliveryFee: deliveryFees[delivery], applied });
  const { discount, shipping, total } = totals;

  if (!hydrated || !authChecked) return <Skeleton />;
  if (enriched.length === 0) return <EmptyBag t={t} />;
  if (!authed) return <SignInWall t={t} />;

  async function ensureAddress(): Promise<string | null> {
    if (!addingAddress && selectedAddress) return selectedAddress;
    if (!newAddr.recipient_name || !newAddr.street || !newAddr.city) {
      setError(t("checkout.error.address"));
      return null;
    }
    try {
      const created = await apiFetch<Address>("/users/me/addresses", {
        method: "POST",
        body: {
          ...newAddr,
          recipient_phone: newAddr.recipient_phone || undefined,
          is_default: addresses.length === 0,
        },
      });
      setAddresses((p) => [...p, created]);
      setSelectedAddress(created.id);
      setAddingAddress(false);
      return created.id;
    } catch (err) {
      setError(readErrorMessage(err));
      return null;
    }
  }

  async function placeOrder(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    const missingSku = enriched.find((it) => !it.skuId);
    if (missingSku) {
      setError(t("checkout.error.sku"));
      return;
    }

    setSubmitting(true);
    try {
      const addrId = await ensureAddress();
      if (!addrId) return;

      // 1) Push the local cart to the backend so the order is built from it.
      await clearCart().catch(() => undefined);
      for (const it of enriched) {
        await addCartItem(it.skuId as string, it.qty);
      }

      // 2) Create the order.
      const { order } = await createOrder({
        delivery_address_id: addrId,
        delivery_method: delivery,
        payment_method: payment,
      });

      // 3) Record the payment intent. For cash on delivery this books a PENDING
      //    payment row against the order so the ledger still balances once the
      //    courier collects — it never redirects or prompts the customer.
      try {
        await initiatePayment({ order_id: order.id, provider: payment });
      } catch (err) {
        // The order stands regardless; the payment row can be reconciled later.
        console.warn("payment initiation failed", err);
      }

      // 4) Clear local cart + coupon, route to confirmation.
      clearLocalCart();
      setCoupon(null);

      router.push(`/checkout/confirmation?order=${encodeURIComponent(order.order_number)}`);
    } catch (err) {
      setError(readErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main>
      <section className="container-hf" style={{ padding: "40px 0 0" }}>
        <Link
          href="/bag"
          style={{
            background: "none",
            border: "none",
            color: "var(--fg-secondary)",
            fontSize: 13,
            display: "inline-flex",
            gap: 6,
            alignItems: "center",
          }}
        >
          <ArrowLeft size={14} strokeWidth={1.5} /> {t("checkout.back_bag")}
        </Link>
        <h1
          style={{
            fontFamily: "var(--font-display)",
            fontWeight: 500,
            fontSize: "clamp(32px,4vw,44px)",
            lineHeight: 1.1,
            marginTop: 14,
          }}
        >
          {t("checkout.title")}
        </h1>
        <div className="gold-rule" style={{ marginTop: 14 }} />
      </section>

      <form onSubmit={placeOrder} className="container-hf checkout-grid">
        <div style={{ display: "flex", flexDirection: "column", gap: 40 }}>
          <Step num="1" eyebrow={t("checkout.step1_eyebrow")} title={t("checkout.step1_title")}>
            {addresses.length > 0 && !addingAddress ? (
              <div className="addr-grid">
                {addresses.map((a) => (
                  <label
                    key={a.id}
                    className={`addr-card${selectedAddress === a.id ? " selected" : ""}`}
                    style={{ cursor: "pointer" }}
                  >
                    <input
                      type="radio"
                      name="address"
                      checked={selectedAddress === a.id}
                      onChange={() => setSelectedAddress(a.id)}
                      style={{ position: "absolute", opacity: 0 }}
                    />
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "space-between",
                        alignItems: "flex-start",
                      }}
                    >
                      <div className="eyebrow">{a.label}</div>
                      {a.is_default && <span className="addr-default">{t("checkout.addr.default")}</span>}
                    </div>
                    <div style={{ fontFamily: "var(--font-display)", fontSize: 18, fontWeight: 500, marginTop: 8 }}>
                      {a.recipient_name}
                    </div>
                    <div style={{ fontSize: 13, color: "var(--fg-secondary)", lineHeight: 1.5 }}>
                      {a.street}
                      <br />
                      {a.city}, {a.country}
                      {a.recipient_phone && (
                        <>
                          <br />
                          <span style={{ fontFamily: "var(--font-mono)" }}>{a.recipient_phone}</span>
                        </>
                      )}
                    </div>
                  </label>
                ))}
                <button
                  type="button"
                  className="addr-add"
                  onClick={() => setAddingAddress(true)}
                >
                  <span style={{ fontSize: 24, lineHeight: 1 }}>+</span>
                  <span>{t("checkout.addr.add_another")}</span>
                </button>
              </div>
            ) : (
              <div className="addr-form-grid">
                <FieldText
                  label={t("checkout.addr.recipient")}
                  value={newAddr.recipient_name}
                  onChange={(v) => setNewAddr((s) => ({ ...s, recipient_name: v }))}
                />
                <FieldText
                  label={t("checkout.addr.label")}
                  value={newAddr.label}
                  onChange={(v) => setNewAddr((s) => ({ ...s, label: v }))}
                />
                <div style={{ gridColumn: "1 / -1" }}>
                  <FieldText
                    label={t("checkout.addr.street")}
                    value={newAddr.street}
                    onChange={(v) => setNewAddr((s) => ({ ...s, street: v }))}
                  />
                </div>
                <FieldText
                  label={t("checkout.addr.city")}
                  value={newAddr.city}
                  onChange={(v) => setNewAddr((s) => ({ ...s, city: v }))}
                />
                <FieldText
                  label={t("checkout.addr.country")}
                  value={newAddr.country}
                  onChange={(v) =>
                    setNewAddr((s) => ({ ...s, country: v.toUpperCase().slice(0, 2) }))
                  }
                />
                <div style={{ gridColumn: "1 / -1" }}>
                  <FieldText
                    label={t("checkout.addr.phone")}
                    value={newAddr.recipient_phone}
                    onChange={(v) => setNewAddr((s) => ({ ...s, recipient_phone: v }))}
                  />
                </div>
                {addresses.length > 0 && (
                  <div style={{ gridColumn: "1 / -1" }}>
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setAddingAddress(false)}
                    >
                      {t("checkout.use_saved")}
                    </button>
                  </div>
                )}
              </div>
            )}
          </Step>

          <Step num="2" eyebrow={t("checkout.step2_eyebrow")} title={t("checkout.step2_title")}>
            <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
              <RadioCard
                name="delivery"
                checked={delivery === "STANDARD"}
                onChange={() => setDelivery("STANDARD")}
                title={t("checkout.delivery.standard")}
                sub={t("checkout.delivery.standard_sub")}
                price={deliveryFees.STANDARD === 0 ? t("common.complimentary") : formatPrice(deliveryFees.STANDARD)}
              />
              <RadioCard
                name="delivery"
                checked={delivery === "EXPRESS"}
                onChange={() => setDelivery("EXPRESS")}
                title={t("checkout.delivery.express")}
                sub={t("checkout.delivery.express_sub")}
                price={formatPrice(deliveryFees.EXPRESS)}
              />
              <RadioCard
                name="delivery"
                checked={delivery === "SAME_DAY"}
                onChange={() => setDelivery("SAME_DAY")}
                title={t("checkout.delivery.sameday")}
                sub={t("checkout.delivery.sameday_sub")}
                price={formatPrice(deliveryFees.SAME_DAY)}
              />
            </div>
          </Step>

          <Step num="3" eyebrow={t("checkout.step3_eyebrow")} title={t("checkout.step3_title")}>
            {/* ── Online payment methods — temporarily disabled ───────────────
                Restore the `payment` state hook above, then un-comment this
                block to bring MTN MoMo, Orange Money and Stripe back. The API
                accepts all four providers already.

            <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
              <RadioCard
                name="payment"
                checked={payment === "mtn_momo"}
                onChange={() => setPayment("mtn_momo")}
                title={t("checkout.payment.mtn")}
                sub={t("checkout.payment.mtn_sub")}
              />
              <RadioCard
                name="payment"
                checked={payment === "orange_money"}
                onChange={() => setPayment("orange_money")}
                title={t("checkout.payment.orange")}
                sub={t("checkout.payment.orange_sub")}
              />
              <RadioCard
                name="payment"
                checked={payment === "stripe"}
                onChange={() => setPayment("stripe")}
                title={t("checkout.payment.card")}
                sub={t("checkout.payment.card_sub")}
              />
              <RadioCard
                name="payment"
                checked={payment === "cash_on_delivery"}
                onChange={() => setPayment("cash_on_delivery")}
                title={t("checkout.payment.cod")}
                sub={t("checkout.payment.cod_sub")}
              />
            </div>
            {(payment === "mtn_momo" || payment === "orange_money") && (
              <div style={{ marginTop: 16, maxWidth: 320 }}>
                <FieldText label={t("checkout.payment.momo_number")} value={momoPhone} onChange={setMomoPhone} />
              </div>
            )}
            ─────────────────────────────────────────────────────────────── */}

            <CodPanel t={t} amount={formatPrice(total)} />
          </Step>
        </div>

        <aside>
          <div className="summary" style={{ position: "sticky", top: 120 }}>
            <div className="eyebrow" style={{ marginBottom: 6 }}>
              {t("checkout.summary.title")}
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 14, marginBottom: 6 }}>
              {enriched.map((it, i) => (
                <div key={i} style={{ display: "flex", gap: 12, alignItems: "center" }}>
                  <div
                    style={{
                      position: "relative",
                      width: 48,
                      height: 48,
                      borderRadius: 4,
                      overflow: "hidden",
                      background: "var(--hf-cream-100)",
                      flexShrink: 0,
                    }}
                  >
                    <Image
                      src={it.product.image}
                      alt={it.product.name}
                      fill
                      sizes="48px"
                      style={{ objectFit: "cover" }}
                    />
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        fontFamily: "var(--font-display)",
                        fontSize: 14,
                        fontWeight: 500,
                      }}
                    >
                      {it.product.name}
                    </div>
                    <div style={{ fontSize: 11, color: "var(--fg-muted)" }}>
                      EU {it.size} · {it.color.name} · ×{it.qty}
                    </div>
                  </div>
                  <div className="t-price" style={{ fontSize: 13 }}>
                    {formatPrice(it.product.price * it.qty)}
                  </div>
                </div>
              ))}
            </div>
            <div className="divider" />
            <div className="row">
              <span>{t("bag.subtotal")}</span>
              <span>{formatPrice(subtotal)}</span>
            </div>
            {discount > 0 && applied?.coupon && (
              <div className="discount-line">
                <span className="name">− {applied.coupon.code}</span>
                <span>− {formatPrice(discount)}</span>
              </div>
            )}
            <div className="row muted">
              <span>{t("bag.courier")}</span>
              <span>{shipping === 0 ? t("common.complimentary") : formatPrice(shipping)}</span>
            </div>

            <CouponBlock
              subtotal={subtotal}
              applied={applied}
              onApply={setCoupon}
              onClear={() => setCoupon(null)}
              compact
            />

            <div className="divider" />
            <div className="row total">
              <span>{t("bag.total")}</span>
              <span>{formatPrice(total)}</span>
            </div>

            {error && (
              <p
                style={{
                  fontFamily: "var(--font-body)",
                  fontSize: 13,
                  color: "var(--hf-danger)",
                  background: "#FCEDED",
                  border: "1px solid #F0C5C5",
                  borderRadius: 6,
                  padding: "10px 12px",
                  margin: 0,
                }}
              >
                {error}
              </p>
            )}

            <button
              type="submit"
              className="btn btn-primary btn-lg btn-block"
              style={{ marginTop: 4 }}
              disabled={submitting}
            >
              {submitting ? t("checkout.cta.placing") : `${t("checkout.cta.place_order")} · ${formatPrice(total)}`}
            </button>
            <p
              style={{
                fontSize: 11,
                color: "var(--fg-muted)",
                textAlign: "center",
                marginTop: 8,
                fontStyle: "italic",
                fontFamily: "var(--font-serif)",
              }}
            >
              {t("checkout.terms")}
            </p>
          </div>
        </aside>
      </form>
    </main>
  );
}

function Step({
  num,
  eyebrow,
  title,
  children,
}: {
  num: string;
  eyebrow: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="step-head">
        <div className="step-num">{num}</div>
        <div>
          <div className="eyebrow">{eyebrow}</div>
          <h3
            style={{
              fontFamily: "var(--font-display)",
              fontSize: 22,
              fontWeight: 500,
              marginTop: 4,
            }}
          >
            {title}
          </h3>
        </div>
      </div>
      {children}
    </section>
  );
}

function FieldText({
  label,
  value,
  onChange,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
}) {
  return (
    <label className="field-label">
      <span>{label}</span>
      <input
        type={type}
        className="hf-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

/**
 * CodPanel replaces the payment-method chooser while online payment is off.
 * It states the amount, when it is due, and what the courier will accept, so
 * the customer is never left wondering how the order gets paid for.
 */
function CodPanel({ t, amount }: { t: T; amount: string }) {
  return (
    <div className="cod-panel">
      <div className="cod-panel-head">
        <span className="cod-badge">
          <Banknote size={15} strokeWidth={1.5} />
        </span>
        <div>
          <div className="eyebrow">{t("checkout.payment.cod")}</div>
          <h4 className="cod-title">{t("checkout.payment.cod_only_title")}</h4>
        </div>
      </div>

      <div className="cod-amount-row">
        <span className="cod-amount-label">{t("checkout.payment.cod_due")}</span>
        <span className="t-price cod-amount">{amount}</span>
      </div>

      <ul className="cod-list">
        <li>{t("checkout.payment.cod_point_1")}</li>
        <li>{t("checkout.payment.cod_point_2")}</li>
        <li>{t("checkout.payment.cod_point_3")}</li>
      </ul>

      <p className="cod-note">{t("checkout.payment.cod_disabled_note")}</p>
    </div>
  );
}

function RadioCard({
  name,
  checked,
  onChange,
  title,
  sub,
  price,
}: {
  name: string;
  checked: boolean;
  onChange: () => void;
  title: string;
  sub: string;
  price?: string;
}) {
  return (
    <label className={`radio-row${checked ? " active" : ""}`}>
      <input type="radio" name={name} checked={checked} onChange={onChange} />
      <div style={{ flex: 1 }}>
        <div className="title">{title}</div>
        <div className="sub">{sub}</div>
      </div>
      {price && <div className="t-price" style={{ fontSize: 16 }}>{price}</div>}
    </label>
  );
}

type T = (key: string, vars?: Record<string, string | number>) => string;

function EmptyBag({ t }: { t: T }) {
  return (
    <section
      className="container-hf"
      style={{ padding: "80px 0", textAlign: "center", maxWidth: 560 }}
    >
      <div className="eyebrow">{t("checkout.title")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(36px,5vw,48px)",
          marginTop: 14,
          lineHeight: 1.1,
        }}
      >
        {t("checkout.empty_title")}
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
        {t("checkout.empty_lede")}
      </p>
      <Link href="/shop/sneakers" className="btn btn-primary btn-lg" style={{ marginTop: 28 }}>
        {t("checkout.empty_cta")}
      </Link>
    </section>
  );
}

function SignInWall({ t }: { t: T }) {
  return (
    <section
      className="container-hf"
      style={{ padding: "80px 0", textAlign: "center", maxWidth: 560 }}
    >
      <div className="eyebrow">{t("checkout.title")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(36px,5vw,48px)",
          marginTop: 14,
          lineHeight: 1.1,
        }}
      >
        {t("checkout.signin_title")}
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
        {t("checkout.signin_lede")}
      </p>
      <div style={{ display: "flex", gap: 12, justifyContent: "center", marginTop: 28 }}>
        <Link href="/account" className="btn btn-primary">
          {t("auth.tab.signin")}
        </Link>
        <Link href="/bag" className="btn btn-secondary">
          {t("checkout.back_bag")}
        </Link>
      </div>
    </section>
  );
}

function Skeleton() {
  return (
    <section className="container-hf" style={{ padding: "80px 0" }}>
      <div
        style={{
          height: 280,
          background: "var(--bg-sunken)",
          borderRadius: 12,
          opacity: 0.4,
        }}
      />
    </section>
  );
}
