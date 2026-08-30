"use client";

import { STORE } from "@/lib/store";
import { useT } from "@/lib/i18n/context";

/**
 * Templated WhatsApp contact block.
 *
 * The message body is composed here rather than by each caller so every entry
 * point — confirmation page, order history, product help — opens a chat that
 * already identifies the customer and what they are asking about. That is the
 * whole point of the deep link: the merchant should never have to ask "which
 * order?" as the first reply.
 */
export type WhatsAppCtaProps = {
  /** Order number to reference, when the caller has one. */
  orderNumber?: string;
  /** Pre-formatted order total, e.g. "XAF 145,000". */
  amount?: string;
  /** `dark` sits on the navy confirmation hero; `light` on a cream page. */
  tone?: "dark" | "light";
  /** Hide the surrounding card and render the button alone. */
  bare?: boolean;
  className?: string;
};

/** Builds the pre-filled message. Exported so tests and emails can reuse it. */
export function buildWhatsAppMessage(opts: {
  brand: string;
  orderNumber?: string;
  amount?: string;
  greeting: string;
  withOrder: string;
  withoutOrder: string;
}): string {
  const { brand, orderNumber, amount, greeting, withOrder, withoutOrder } = opts;
  const head = greeting.replace("{{brand}}", brand);
  const body = orderNumber
    ? withOrder
        .replace("{{order}}", orderNumber)
        .replace("{{amount}}", amount ?? "")
        // Collapse the separator when no amount was supplied.
        .replace(" ()", "")
    : withoutOrder;
  return `${head}\n\n${body}`;
}

export function WhatsAppCta({
  orderNumber,
  amount,
  tone = "light",
  bare = false,
  className,
}: WhatsAppCtaProps) {
  const { t } = useT();

  const message = buildWhatsAppMessage({
    brand: t("common.brand"),
    orderNumber,
    amount,
    greeting: t("wa.cta.greeting"),
    withOrder: t("wa.cta.with_order"),
    withoutOrder: t("wa.cta.without_order"),
  });

  const href = `https://wa.me/${STORE.whatsappNumber}?text=${encodeURIComponent(message)}`;

  const button = (
    <a
      className="wa-cta-btn"
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={t("wa.cta.button")}
    >
      <WhatsAppGlyph />
      <span>{t("wa.cta.button")}</span>
    </a>
  );

  if (bare) return <div className={className}>{button}</div>;

  return (
    <section className={`wa-cta wa-cta-${tone}${className ? ` ${className}` : ""}`}>
      <div className="eyebrow wa-cta-eyebrow">{t("wa.cta.eyebrow")}</div>
      <h3 className="wa-cta-title">{t("wa.cta.title")}</h3>
      <p className="wa-cta-body">{t("wa.cta.body")}</p>
      {button}
      <p className="wa-cta-meta">
        <span className="wa-cta-number">{STORE.whatsappDisplay}</span>
        <span aria-hidden>·</span>
        <span>{t("wa.cta.hours")}</span>
      </p>
    </section>
  );
}

function WhatsAppGlyph() {
  return (
    <svg width="19" height="19" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M20.5 3.5A11.45 11.45 0 0 0 12 0C5.4 0 .05 5.35.05 11.95c0 2.1.55 4.15 1.6 5.95L0 24l6.3-1.65a11.9 11.9 0 0 0 5.7 1.45h.01c6.6 0 11.95-5.35 11.95-11.95 0-3.2-1.25-6.2-3.46-8.35zM17.5 14.4c-.3-.15-1.75-.85-2-.95-.3-.1-.45-.15-.65.15-.2.3-.75.95-.9 1.15-.15.15-.3.2-.6.05-.3-.15-1.25-.45-2.4-1.45-.9-.8-1.5-1.75-1.65-2.05-.15-.3 0-.45.15-.6.15-.15.3-.35.45-.5.15-.15.2-.3.3-.5.1-.2.05-.35-.05-.5-.05-.15-.65-1.6-.9-2.2-.25-.6-.5-.5-.65-.5h-.55c-.2 0-.5.05-.75.35-.25.3-1 1-1 2.45s1 2.85 1.15 3.05c.15.2 2 3.1 4.85 4.35.7.3 1.25.5 1.65.6.7.25 1.35.2 1.85.1.55-.1 1.75-.7 2-1.4.25-.7.25-1.3.15-1.4-.05-.1-.25-.15-.55-.3z" />
    </svg>
  );
}
