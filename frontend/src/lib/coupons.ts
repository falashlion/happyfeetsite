// Promo codes mirrored from the design bundle. Pure client-side — labels and
// descriptions are translated via the i18n dict (`coupon.<CODE>.label` etc.),
// so the same code surface yields EN or FR text without code changes.

export type Coupon =
  | { code: string; kind: "percent"; amount: number; minSubtotal?: number; members?: boolean }
  | { code: string; kind: "flat"; amount: number; minSubtotal?: number }
  | { code: string; kind: "shipping" };

export const HF_COUPONS: Record<string, Coupon> = {
  WELCOME10: { code: "WELCOME10", kind: "percent", amount: 10 },
  ATELIER20: { code: "ATELIER20", kind: "percent", amount: 20, minSubtotal: 30000 },
  FREESHIP:  { code: "FREESHIP",  kind: "shipping" },
  XAF5000:   { code: "XAF5000",   kind: "flat", amount: 5000, minSubtotal: 25000 },
  GOLD25:    { code: "GOLD25",    kind: "percent", amount: 25, members: true },
  REFRIEND:  { code: "REFRIEND",  kind: "flat", amount: 10000, minSubtotal: 40000 },
};

export type CouponResolution =
  | { ok: true; coupon: Coupon; discount: number; shippingFree: boolean }
  | { ok: false; errorKey: string; errorVars?: Record<string, string | number> };

export function applyCoupon(
  code: string,
  ctx: { subtotal: number; isMember?: boolean },
): CouponResolution {
  if (!code) return { ok: false, errorKey: "coupon.error.empty" };
  const key = code.trim().toUpperCase();
  const c = HF_COUPONS[key];
  if (!c) return { ok: false, errorKey: "coupon.error.unknown", errorVars: { code: key } };
  if ("minSubtotal" in c && c.minSubtotal && ctx.subtotal < c.minSubtotal) {
    return {
      ok: false,
      errorKey: "coupon.error.min",
      errorVars: { amount: formatXAF(c.minSubtotal - ctx.subtotal) },
    };
  }
  if (c.kind === "percent" && c.members && !ctx.isMember) {
    return { ok: false, errorKey: "coupon.error.members" };
  }
  if (c.kind === "percent") {
    return { ok: true, coupon: c, discount: Math.round((ctx.subtotal * c.amount) / 100), shippingFree: false };
  }
  if (c.kind === "flat") {
    return { ok: true, coupon: c, discount: Math.min(ctx.subtotal, c.amount), shippingFree: false };
  }
  return { ok: true, coupon: c, discount: 0, shippingFree: true };
}

function formatXAF(v: number): string {
  return `XAF ${v.toLocaleString("en")}`;
}

export type AppliedCoupon = { coupon: Coupon; discount: number; shippingFree: boolean };

export function computeTotals(args: {
  subtotal: number;
  deliveryFee: number;
  applied: AppliedCoupon | null;
}): { subtotal: number; discount: number; shipping: number; total: number } {
  const { subtotal, deliveryFee } = args;
  let discount = 0;
  let shipping = deliveryFee;
  const c = args.applied?.coupon;
  if (c) {
    if (c.kind === "percent") discount = Math.round((subtotal * c.amount) / 100);
    else if (c.kind === "flat") discount = Math.min(subtotal, c.amount);
    else if (c.kind === "shipping") shipping = 0;
  }
  const total = Math.max(0, subtotal - discount) + shipping;
  return { subtotal, discount, shipping, total };
}
