"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useRouter } from "next/navigation";
import {
  DEFAULT_LOCALE,
  DICT,
  isLocale,
  translate,
  type Locale,
  type Vars,
} from "./dict";

const COOKIE = "hf_locale";

type LocaleContextValue = {
  locale: Locale;
  setLocale: (l: Locale) => void;
  t: (key: string, vars?: Vars) => string;
};

const LocaleContext = createContext<LocaleContextValue | null>(null);

function readCookie(): Locale | null {
  if (typeof document === "undefined") return null;
  const m = document.cookie.match(/(?:^|;\s*)hf_locale=([^;]+)/);
  if (!m) return null;
  const v = decodeURIComponent(m[1]);
  return isLocale(v) ? v : null;
}

function writeCookie(l: Locale) {
  if (typeof document === "undefined") return;
  // 1 year, site-wide, lax for navigation.
  const expires = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toUTCString();
  document.cookie = `${COOKIE}=${l}; expires=${expires}; path=/; samesite=lax`;
}

export function LocaleProvider({
  initialLocale = DEFAULT_LOCALE,
  children,
}: {
  initialLocale?: Locale;
  children: ReactNode;
}) {
  const router = useRouter();
  const [locale, setLocaleState] = useState<Locale>(initialLocale);

  // On mount, prefer the cookie (set by a previous session) over the initial
  // server-rendered value so we stay consistent across tabs.
  useEffect(() => {
    const fromCookie = readCookie();
    if (fromCookie && fromCookie !== locale) {
      setLocaleState(fromCookie);
    }
    if (typeof document !== "undefined") {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  const setLocale = useCallback(
    (l: Locale) => {
      if (l === locale) return;
      writeCookie(l);
      setLocaleState(l);
      if (typeof document !== "undefined") {
        document.documentElement.lang = l;
      }
      // Server components read the cookie at render — refresh re-renders them.
      router.refresh();
    },
    [locale, router],
  );

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale,
      t: (key, vars) => translate(locale, key, vars),
    }),
    [locale, setLocale],
  );

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

export function useLocale() {
  const ctx = useContext(LocaleContext);
  if (!ctx) throw new Error("useLocale must be used inside <LocaleProvider>");
  return ctx;
}

// Convenience hook — same shape as the design's useT().
export function useT() {
  const { t, locale } = useLocale();
  return { t, locale, dict: DICT[locale] };
}
