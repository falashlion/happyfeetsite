// Server-side locale helpers. Reads the `hf_locale` cookie on each request so
// server components can render the correct language; pair with a route refresh
// when the locale changes (handled by the client toggle).

import { cookies } from "next/headers";
import { DEFAULT_LOCALE, isLocale, translate, type Locale, type Vars } from "./dict";

export const LOCALE_COOKIE = "hf_locale";

export async function getLocale(): Promise<Locale> {
  const store = await cookies();
  const v = store.get(LOCALE_COOKIE)?.value;
  return isLocale(v) ? v : DEFAULT_LOCALE;
}

// Server-side translator. Use in server components and Next.js `generateMetadata`.
export async function tServer(): Promise<(key: string, vars?: Vars) => string> {
  const locale = await getLocale();
  return (key, vars) => translate(locale, key, vars);
}
