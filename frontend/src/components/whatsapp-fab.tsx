"use client";

import { useState } from "react";
import { LazyMotion, domAnimation, m, AnimatePresence } from "framer-motion";
import { X, Mail, Truck, Info, RotateCcw, Sparkles } from "lucide-react";
import { useT } from "@/lib/i18n/context";
import { STORE } from "@/lib/store";

const TOPICS = [
  { id: "order-tracking", labelKey: "wa.topic.order_tracking", icon: Truck },
  { id: "sizing",         labelKey: "wa.topic.sizing",         icon: Info },
  { id: "returns",        labelKey: "wa.topic.returns",        icon: RotateCcw },
  { id: "concierge",      labelKey: "wa.topic.concierge",      icon: Sparkles },
  { id: "general",        labelKey: "wa.topic.general",        icon: Mail },
] as const;

type T = (key: string, vars?: Record<string, string | number>) => string;

function buildMessage(t: T, locale: string, kind: string, body?: string) {
  const stamp = new Date().toLocaleString(locale === "fr" ? "fr-FR" : "en-GB", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
  const header = `${t("wa.msg.header")}\n\n`;
  const footer = `\n\n— ${t("wa.msg.footer_sent_from")} · ${stamp}`;
  switch (kind) {
    case "sizing":
      return header + t("wa.msg.sizing") + footer;
    case "concierge":
      return header + t("wa.msg.concierge") + footer;
    case "order-tracking":
      return header + t("wa.msg.order_tracking") + footer;
    case "returns":
      return header + t("wa.msg.returns") + footer;
    case "support":
      return header + `${t("wa.msg.support_intro")}\n${body || "—"}` + footer;
    default:
      return header + t("wa.msg.general") + footer;
  }
}

export function WhatsAppFab() {
  const { t, locale } = useT();
  const [openPanel, setOpenPanel] = useState(false);
  const [msg, setMsg] = useState("");

  function openWA(kind: string, body?: string) {
    const url = `https://wa.me/${STORE.whatsappNumber}?text=${encodeURIComponent(buildMessage(t, locale, kind, body))}`;
    window.open(url, "_blank", "noopener,noreferrer");
  }

  return (
    <LazyMotion features={domAnimation} strict>
      <m.button
        whileHover={{ y: -2 }}
        whileTap={{ scale: 0.98 }}
        className="wa-fab"
        aria-label={t("wa.fab_aria")}
        onClick={() => setOpenPanel((o) => !o)}
      >
        {openPanel ? (
          <X size={22} strokeWidth={1.5} />
        ) : (
          <WhatsAppGlyph />
        )}
      </m.button>

      <AnimatePresence>
        {openPanel && (
          <m.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 16 }}
            transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
            role="dialog"
            aria-label={t("wa.dialog_aria")}
            className="fixed right-6 bottom-24 z-[1000] w-[min(340px,90vw)] rounded-2xl overflow-hidden shadow-[0_24px_64px_rgba(14,27,58,.28)]"
            style={{ background: "#fff", border: "1px solid var(--border-hair)" }}
          >
            <header
              style={{
                background: "var(--hf-navy-900)",
                color: "var(--hf-cream-50)",
                padding: "20px 22px",
              }}
            >
              <div className="eyebrow" style={{ color: "var(--hf-gold-300)" }}>
                {t("wa.eyebrow")}
              </div>
              <h4
                style={{
                  fontFamily: "var(--font-display)",
                  fontWeight: 500,
                  fontSize: 22,
                  marginTop: 6,
                }}
              >
                {t("wa.title")}
              </h4>
              <p style={{ fontSize: 13, marginTop: 6, color: "rgba(245,239,224,.7)" }}>
                {t("wa.hours")}
              </p>
            </header>
            <div style={{ padding: 18 }}>
              <div className="flex flex-col gap-1">
                {TOPICS.map((topic) => {
                  const Icon = topic.icon;
                  return (
                    <button
                      key={topic.id}
                      onClick={() => openWA(topic.id)}
                      className="flex items-center gap-3 text-left px-3 py-3 rounded text-[13px]"
                      style={{ background: "transparent", border: "none", cursor: "pointer" }}
                    >
                      <Icon size={16} strokeWidth={1.5} style={{ color: "var(--hf-gold-600)" }} />
                      <span style={{ flex: 1 }}>{t(topic.labelKey)}</span>
                    </button>
                  );
                })}
              </div>
              <div
                style={{
                  textAlign: "center",
                  fontSize: 10,
                  letterSpacing: ".22em",
                  textTransform: "uppercase",
                  color: "var(--fg-muted)",
                  margin: "14px 0",
                }}
              >
                {t("wa.separator")}
              </div>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  openWA("support", msg);
                  setMsg("");
                  setOpenPanel(false);
                }}
              >
                <textarea
                  rows={3}
                  value={msg}
                  onChange={(e) => setMsg(e.target.value)}
                  placeholder={t("wa.placeholder")}
                  style={{
                    width: "100%",
                    border: "1px solid var(--border-soft)",
                    borderRadius: 4,
                    padding: 10,
                    fontSize: 13,
                    fontFamily: "var(--font-body)",
                    outline: "none",
                    resize: "vertical",
                  }}
                />
                <button
                  type="submit"
                  className="btn btn-primary btn-block"
                  style={{ marginTop: 10 }}
                >
                  {t("wa.send")}
                </button>
              </form>
            </div>
          </m.div>
        )}
      </AnimatePresence>
    </LazyMotion>
  );
}

function WhatsAppGlyph() {
  return (
    <svg width="26" height="26" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M20.5 3.5A11.45 11.45 0 0 0 12 0C5.4 0 .05 5.35.05 11.95c0 2.1.55 4.15 1.6 5.95L0 24l6.3-1.65a11.9 11.9 0 0 0 5.7 1.45h.01c6.6 0 11.95-5.35 11.95-11.95 0-3.2-1.25-6.2-3.46-8.35zM17.5 14.4c-.3-.15-1.75-.85-2-.95-.3-.1-.45-.15-.65.15-.2.3-.75.95-.9 1.15-.15.15-.3.2-.6.05-.3-.15-1.25-.45-2.4-1.45-.9-.8-1.5-1.75-1.65-2.05-.15-.3 0-.45.15-.6.15-.15.3-.35.45-.5.15-.15.2-.3.3-.5.1-.2.05-.35-.05-.5-.05-.15-.65-1.6-.9-2.2-.25-.6-.5-.5-.65-.5h-.55c-.2 0-.5.05-.75.35-.25.3-1 1-1 2.45s1 2.85 1.15 3.05c.15.2 2 3.1 4.85 4.35.7.3 1.25.5 1.65.6.7.25 1.35.2 1.85.1.55-.1 1.75-.7 2-1.4.25-.7.25-1.3.15-1.4-.05-.1-.25-.15-.55-.3z" />
    </svg>
  );
}
