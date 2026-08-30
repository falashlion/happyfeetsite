"use client";

import { useLocale } from "@/lib/i18n/context";
import { LOCALE_LABEL, LOCALE_NAME, SUPPORTED_LOCALES, type Locale } from "@/lib/i18n/dict";

// Pill toggle used in the desktop nav. Two-state (EN | FR) with a sliding
// indicator, mirroring the channel-toggle styling from the design.
export function LocaleToggle({ variant = "pill" }: { variant?: "pill" | "list" }) {
  const { locale, setLocale, t } = useLocale();

  if (variant === "list") {
    // Inline list of buttons — used inside the mobile drawer.
    return (
      <div role="radiogroup" aria-label={t("nav.language")}>
        {SUPPORTED_LOCALES.map((l) => (
          <button
            key={l}
            type="button"
            role="radio"
            aria-checked={locale === l}
            onClick={() => setLocale(l)}
            className="locale-list-row"
            data-active={locale === l}
          >
            <span
              style={{
                fontFamily: "var(--font-mono)",
                fontSize: 11,
                letterSpacing: ".18em",
                color: locale === l ? "var(--hf-gold-600)" : "var(--fg-muted)",
                fontWeight: 600,
                width: 28,
              }}
            >
              {LOCALE_LABEL[l]}
            </span>
            <span>{LOCALE_NAME[l]}</span>
          </button>
        ))}
      </div>
    );
  }

  return (
    <div
      className="locale-toggle"
      role="radiogroup"
      aria-label={t("nav.language")}
    >
      {SUPPORTED_LOCALES.map((l: Locale) => (
        <button
          key={l}
          type="button"
          role="radio"
          aria-checked={locale === l}
          onClick={() => setLocale(l)}
          className={locale === l ? "active" : ""}
        >
          {LOCALE_LABEL[l]}
        </button>
      ))}
      <span
        className="indicator"
        data-locale={locale}
        aria-hidden
      />
    </div>
  );
}
