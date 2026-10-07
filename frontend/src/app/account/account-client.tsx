"use client";

import Image from "next/image";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight, Truck, RotateCcw, Shield, Sparkles } from "lucide-react";
import { login, register, me, logout, type AuthUser } from "@/lib/api/auth";
import { GoogleSignIn } from "@/components/google-sign-in";
import { useT } from "@/lib/i18n/context";

// How long the signed-in confirmation stays on screen before the storefront.
const REDIRECT_SECONDS = 3;

type Mode = "login" | "register";
type Channel = "email" | "phone";

type T = (key: string, vars?: Record<string, string | number>) => string;

const FEATURE_KEYS: { Icon: typeof Truck; key: string }[] = [
  { Icon: Truck, key: "auth.feature.courier" },
  { Icon: RotateCcw, key: "auth.feature.returns" },
  { Icon: Shield, key: "auth.feature.authentic" },
  { Icon: Sparkles, key: "auth.feature.concierge" },
];

function readErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === "object" && "message" in err) {
    const m = (err as { message?: unknown }).message;
    if (typeof m === "string" && m.length > 0) return m;
  }
  return fallback;
}

export function AccountClient() {
  const { t } = useT();
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("login");
  const [channel, setChannel] = useState<Channel>("email");
  const [user, setUser] = useState<AuthUser | null>(null);
  const [hydrated, setHydrated] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Seconds left before we send a freshly signed-in customer to the storefront.
  // Null means no redirect pending.
  const [redirectIn, setRedirectIn] = useState<number | null>(null);
  const redirectTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Let the signed-in state land visibly before navigating away: going straight
  // to the home page makes a successful sign-in look like nothing happened.
  const goHomeShortly = useCallback(() => {
    setRedirectIn(REDIRECT_SECONDS);
    if (redirectTimer.current) clearInterval(redirectTimer.current);
    redirectTimer.current = setInterval(() => {
      setRedirectIn((n) => {
        if (n === null) return null;
        if (n <= 1) {
          if (redirectTimer.current) clearInterval(redirectTimer.current);
          router.push("/");
          return 0;
        }
        return n - 1;
      });
    }, 1000);
  }, [router]);

  // An unmount mid-countdown must not leave a timer pushing a route later.
  useEffect(
    () => () => {
      if (redirectTimer.current) clearInterval(redirectTimer.current);
    },
    [],
  );

  useEffect(() => {
    let cancelled = false;
    me().then((u) => {
      if (cancelled) return;
      setUser(u);
      setHydrated(true);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    const fd = new FormData(e.currentTarget);
    const password = String(fd.get("password") ?? "");
    const contact = channel === "email"
      ? { email: String(fd.get("email") ?? "").trim() }
      : { phone: String(fd.get("phone") ?? "").trim() };
    setBusy(true);
    try {
      if (mode === "register") {
        const first_name = String(fd.get("first_name") ?? "").trim();
        const last_name = String(fd.get("last_name") ?? "").trim();
        if (!first_name || !last_name) throw new Error(t("auth.error.both_names"));
        await register({ first_name, last_name, password, ...contact });
      }
      const res = await login({ password, ...contact });
      setUser(res.user);
      router.refresh();
      goHomeShortly();
    } catch (err) {
      setError(readErrorMessage(err, t("auth.error.generic")));
    } finally {
      setBusy(false);
    }
  }

  const onGoogleSignedIn = useCallback(
    (u: AuthUser) => {
      setError(null);
      setUser(u);
      router.refresh();
      goHomeShortly();
    },
    [router, goHomeShortly],
  );

  const onGoogleError = useCallback(
    (message: string) => setError(message || t("auth.google.error")),
    [t],
  );

  async function onSignOut() {
    setBusy(true);
    try {
      await logout();
      setUser(null);
    } finally {
      setBusy(false);
    }
  }

  if (hydrated && user) {
    return <SignedIn t={t} user={user} onSignOut={onSignOut} busy={busy} redirectIn={redirectIn} />;
  }

  const isLogin = mode === "login";

  return (
    <main className="auth-shell">
      {/* Editorial side panel */}
      <aside className="auth-aside">
        <div className="auth-aside-inner">
          <Image
            src="/brand/logo-mark-navy.png"
            alt={t("common.brand")}
            width={64}
            height={64}
            className="auth-seal"
          />
          <div className="eyebrow" style={{ color: "var(--hf-gold-300)" }}>
            {isLogin ? t("auth.eyebrow.welcome") : t("auth.eyebrow.member")}
          </div>
          <h1 className="auth-h1" style={{ whiteSpace: "pre-line" }}>
            {isLogin ? t("auth.side.welcome") : t("auth.side.member")}
          </h1>
          <p className="auth-lede">
            {isLogin ? t("auth.side.welcome_lede") : t("auth.side.member_lede")}
          </p>
          <ul className="auth-list">
            {FEATURE_KEYS.map(({ Icon, key }) => (
              <li key={key}>
                <Icon size={18} strokeWidth={1.5} />
                <span>{t(key)}</span>
              </li>
            ))}
          </ul>
        </div>
      </aside>

      {/* Form side */}
      <section className="auth-form-wrap">
        <div className="auth-form-inner">
          <div className="auth-tabs" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={isLogin}
              className={isLogin ? "active" : ""}
              onClick={() => setMode("login")}
            >
              {t("auth.tab.signin")}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={!isLogin}
              className={!isLogin ? "active" : ""}
              onClick={() => setMode("register")}
            >
              {t("auth.tab.register")}
            </button>
          </div>

          <h2 className="auth-form-title">
            {isLogin ? t("auth.form_title.signin") : t("auth.form_title.register")}
          </h2>
          <p className="auth-form-sub">
            {isLogin ? t("auth.form_sub.signin") : t("auth.form_sub.register")}
          </p>

          <div className="channel-toggle" role="tablist" aria-label={t("auth.channel.email") + " / " + t("auth.channel.phone")}>
            <button
              type="button"
              role="tab"
              aria-selected={channel === "email"}
              className={channel === "email" ? "active" : ""}
              onClick={() => setChannel("email")}
            >
              {t("auth.channel.email")}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={channel === "phone"}
              className={channel === "phone" ? "active" : ""}
              onClick={() => setChannel("phone")}
            >
              {t("auth.channel.phone")}
            </button>
            <span className="indicator" data-channel={channel} />
          </div>

          <form onSubmit={onSubmit} className="auth-form" noValidate>
            {!isLogin && (
              <div className="auth-row two">
                <AuthField
                  name="first_name"
                  label={t("auth.field.first_name")}
                  placeholder={t("auth.placeholder.first_name")}
                  required
                />
                <AuthField
                  name="last_name"
                  label={t("auth.field.last_name")}
                  placeholder={t("auth.placeholder.last_name")}
                  required
                />
              </div>
            )}

            {channel === "email" ? (
              <AuthField
                name="email"
                type="email"
                label={t("auth.field.email")}
                placeholder={t("auth.placeholder.email")}
                required
              />
            ) : (
              <AuthField
                name="phone"
                type="tel"
                label={t("auth.field.phone")}
                placeholder={t("auth.placeholder.phone")}
                required
              />
            )}

            <AuthField
              name="password"
              type="password"
              label={t("auth.field.password")}
              placeholder={isLogin ? t("auth.field.password_placeholder.signin") : t("auth.field.password_placeholder.register")}
              required
              helper={!isLogin ? t("auth.field.password_helper") : null}
            />

            {isLogin && (
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  marginTop: -4,
                  fontSize: 12,
                  flexWrap: "wrap",
                  gap: 8,
                }}
              >
                <label style={{ display: "inline-flex", alignItems: "center", gap: 8, color: "var(--fg-secondary)" }}>
                  <input type="checkbox" defaultChecked style={{ accentColor: "var(--hf-navy-900)" }} />
                  {t("auth.keep_signed")}
                </label>
                <a style={{ color: "var(--fg-link)", cursor: "pointer", textDecoration: "underline" }}>
                  {t("auth.forgot")}
                </a>
              </div>
            )}

            {!isLogin && (
              <label style={{ display: "flex", gap: 10, alignItems: "flex-start", fontSize: 12, color: "var(--fg-secondary)", lineHeight: 1.5 }}>
                <input type="checkbox" defaultChecked style={{ accentColor: "var(--hf-navy-900)", marginTop: 3 }} />
                <span>{t("auth.consent")}</span>
              </label>
            )}

            {error && <div className="auth-error">{error}</div>}

            <button type="submit" className="btn btn-primary btn-lg btn-block" disabled={busy}>
              {busy ? (
                t("common.loading")
              ) : isLogin ? (
                <>
                  {t("auth.cta.signin")} <ArrowRight size={16} strokeWidth={1.5} />
                </>
              ) : (
                <>
                  {t("auth.cta.register")} <ArrowRight size={16} strokeWidth={1.5} />
                </>
              )}
            </button>
          </form>

          {/* Renders nothing unless the API reports Google sign-in configured. */}
          <GoogleSignIn onSignedIn={onGoogleSignedIn} onError={onGoogleError} />

          <p className="auth-footnote">
            {isLogin ? (
              <>
                {t("auth.footnote.to_register")}{" "}
                <a onClick={() => setMode("register")}>{t("auth.footnote.create_link")}</a>
              </>
            ) : (
              <>
                {t("auth.footnote.to_signin")}{" "}
                <a onClick={() => setMode("login")}>{t("auth.footnote.signin_link")}</a>
              </>
            )}
          </p>

          <p className="auth-fineprint">
            {t("auth.fineprint.lead")} <a>{t("auth.fineprint.terms")}</a>{" "}
            {t("auth.fineprint.and")} <a>{t("auth.fineprint.privacy")}</a>.
          </p>
        </div>
      </section>
    </main>
  );
}

function AuthField({
  name,
  label,
  type = "text",
  placeholder,
  required,
  helper,
}: {
  name: string;
  label: string;
  type?: string;
  placeholder?: string;
  required?: boolean;
  helper?: string | null;
}) {
  return (
    <label className="auth-field">
      <span>
        {label}
        {required && <em>*</em>}
      </span>
      <input type={type} name={name} placeholder={placeholder} required={required} autoComplete={autoCompleteFor(name, type)} />
      {helper && <small>{helper}</small>}
    </label>
  );
}

function autoCompleteFor(name: string, type: string) {
  if (name === "email") return "email";
  if (name === "phone") return "tel";
  if (name === "first_name") return "given-name";
  if (name === "last_name") return "family-name";
  if (type === "password") return name === "password" ? "current-password" : "new-password";
  return undefined;
}

function SignedIn({
  t,
  user,
  onSignOut,
  busy,
  redirectIn,
}: {
  t: T;
  user: AuthUser;
  onSignOut: () => void;
  busy: boolean;
  // Seconds until the storefront, or null when the customer arrived here with
  // an existing session and nothing is pending.
  redirectIn: number | null;
}) {
  return (
    <section className="container-hf" style={{ padding: "72px 0", maxWidth: 720 }}>
      <div className="eyebrow">{t("nav.account")}</div>
      <h1
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: 500,
          fontSize: "clamp(32px,4vw,44px)",
          lineHeight: 1.1,
          marginTop: 14,
        }}
      >
        {t("account.signedin.welcome_back", { name: user.first_name || "—" })}
      </h1>
      <div className="gold-rule" style={{ marginTop: 14 }} />

      {redirectIn !== null && (
        <p
          role="status"
          aria-live="polite"
          style={{
            marginTop: 18,
            padding: "10px 14px",
            borderRadius: 999,
            display: "inline-block",
            background: "rgba(176,143,69,.12)",
            color: "var(--ink, #0e1b3a)",
            fontSize: 13,
          }}
        >
          {redirectIn > 0
            ? `Taking you to the store in ${redirectIn}…`
            : "Taking you to the store…"}
        </p>
      )}
      <p
        style={{
          fontFamily: "var(--font-serif)",
          fontStyle: "italic",
          fontSize: 19,
          color: "var(--fg-secondary)",
          marginTop: 24,
        }}
      >
        {t("account.signedin.lede")}
      </p>
      <p style={{ fontSize: 13, color: "var(--fg-muted)", marginTop: 16, fontFamily: "var(--font-mono)" }}>
        {t("account.signedin.meta", {
          contact: user.email ?? user.phone ?? "",
          role: user.role,
        })}
      </p>
      <button
        type="button"
        onClick={onSignOut}
        disabled={busy}
        className="btn btn-secondary"
        style={{ marginTop: 28 }}
      >
        {busy ? t("account.signedin.signing_out") : t("common.signout")}
      </button>
    </section>
  );
}
