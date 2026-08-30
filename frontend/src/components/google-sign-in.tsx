"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { getAuthProviders, loginWithGoogle, type AuthUser } from "@/lib/api/auth";
import { useT } from "@/lib/i18n/context";

/**
 * "Sign in with Google" via Google Identity Services.
 *
 * GIS hands the browser a signed ID token; we forward it to the API, which
 * verifies Google's signature and the audience before minting our own session.
 * Nothing the browser says about *who* the user is is trusted — only the token.
 *
 * The button is Google's own rendered widget rather than a lookalike: their
 * branding guidelines require it, and it carries the accessibility and
 * localisation behaviour for free.
 */

const GIS_SRC = "https://accounts.google.com/gsi/client";

type CredentialResponse = { credential?: string };

type GoogleIdApi = {
  initialize(opts: {
    client_id: string;
    callback: (r: CredentialResponse) => void;
    auto_select?: boolean;
    cancel_on_tap_outside?: boolean;
    use_fedcm_for_prompt?: boolean;
  }): void;
  renderButton(parent: HTMLElement, opts: Record<string, unknown>): void;
};

declare global {
  interface Window {
    google?: { accounts?: { id?: GoogleIdApi } };
  }
}

/** Loads the GIS script once per page, shared by any number of callers. */
let gisPromise: Promise<void> | null = null;
function loadGis(): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  if (window.google?.accounts?.id) return Promise.resolve();
  if (gisPromise) return gisPromise;

  gisPromise = new Promise<void>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${GIS_SRC}"]`);
    if (existing) {
      existing.addEventListener("load", () => resolve());
      existing.addEventListener("error", () => reject(new Error("gis-load-failed")));
      return;
    }
    const s = document.createElement("script");
    s.src = GIS_SRC;
    s.async = true;
    s.defer = true;
    s.onload = () => resolve();
    s.onerror = () => {
      gisPromise = null; // let a later mount retry
      reject(new Error("gis-load-failed"));
    };
    document.head.appendChild(s);
  });
  return gisPromise;
}

export function GoogleSignIn({
  onSignedIn,
  onError,
}: {
  /** Called with the user, and whether this sign-in created the account. */
  onSignedIn: (user: AuthUser, isNew: boolean) => void;
  onError: (message: string) => void;
}) {
  const { t, locale } = useT();
  const slot = useRef<HTMLDivElement>(null);
  const draw = useRef<(() => void) | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "busy" | "off">("loading");

  // Keep the latest callbacks in refs: GIS captures the callback once at
  // initialize() time, so closing over props directly would pin the first render.
  const signedIn = useRef(onSignedIn);
  const errored = useRef(onError);
  useEffect(() => {
    signedIn.current = onSignedIn;
    errored.current = onError;
  }, [onSignedIn, onError]);

  const handleCredential = useCallback(
    async (res: CredentialResponse) => {
      if (!res.credential) {
        errored.current(t("auth.google.error"));
        return;
      }
      setStatus("busy");
      try {
        const out = await loginWithGoogle(res.credential);
        signedIn.current(out.user, out.is_new);
      } catch (err) {
        const msg =
          err && typeof err === "object" && "message" in err
            ? String((err as { message?: unknown }).message)
            : t("auth.google.error");
        errored.current(msg);
        setStatus("ready");
      }
    },
    [t],
  );

  useEffect(() => {
    let cancelled = false;
    let observer: ResizeObserver | undefined;

    (async () => {
      const providers = await getAuthProviders();
      const clientId = providers?.google?.enabled ? providers.google.client_id : undefined;
      if (cancelled) return;
      if (!clientId) {
        setStatus("off"); // API has no client ID configured — render nothing
        return;
      }

      try {
        await loadGis();
      } catch {
        if (!cancelled) setStatus("off"); // offline, or blocked by an extension
        return;
      }
      if (cancelled) return;

      const id = window.google?.accounts?.id;
      if (!id || !slot.current) {
        setStatus("off");
        return;
      }

      id.initialize({
        client_id: clientId,
        callback: handleCredential,
        // Never sign someone in without an explicit click.
        auto_select: false,
        cancel_on_tap_outside: true,
        use_fedcm_for_prompt: true,
      });

      draw.current = () => {
        const host = slot.current;
        if (!host) return;
        // GIS only accepts a pixel width (max 400) and ignores CSS sizing on
        // its shadow-DOM button, so measure the column and render to fit —
        // otherwise the widget overflows a narrow phone viewport.
        const available = host.parentElement?.clientWidth ?? host.clientWidth;
        const width = Math.max(200, Math.min(400, Math.floor(available)));
        host.innerHTML = "";
        id.renderButton(host, {
          type: "standard",
          theme: "outline",
          size: "large",
          shape: "rectangular",
          text: "continue_with",
          logo_alignment: "center",
          locale,
          width,
        });
      };

      draw.current();
      setStatus("ready");

      // Re-render on resize: rotating a phone or opening a desktop devtools
      // pane changes the column width, and the widget cannot reflow itself.
      if (typeof ResizeObserver !== "undefined" && slot.current?.parentElement) {
        let last = slot.current.parentElement.clientWidth;
        observer = new ResizeObserver(() => {
          const now = slot.current?.parentElement?.clientWidth ?? last;
          if (Math.abs(now - last) < 8) return; // ignore sub-pixel churn
          last = now;
          draw.current?.();
        });
        observer.observe(slot.current.parentElement);
      }
    })();

    return () => {
      cancelled = true;
      observer?.disconnect();
    };
  }, [handleCredential, locale]);

  if (status === "off") return null;

  return (
    <div className="google-signin">
      <div className="auth-or">
        <span>{t("auth.or")}</span>
      </div>

      <div className="google-signin-slot" aria-busy={status !== "ready"}>
        <div ref={slot} />
        {status === "loading" && <div className="google-signin-skeleton" aria-hidden />}
        {status === "busy" && (
          <div className="google-signin-busy" role="status">
            {t("auth.google.signing_in")}
          </div>
        )}
      </div>
    </div>
  );
}
