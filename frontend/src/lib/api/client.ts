// Typed HTTP client for the HappyFeet Go backend (REST, /api/v1).
//
// Reads the base URL from NEXT_PUBLIC_API_URL (browser) or HAPPYFEET_API_URL
// (server). Defaults to http://localhost:8080/api/v1.

export type ApiError = {
  code: string;
  message: string;
  status: number;
  requestId?: string;
};

// NEXT_PUBLIC_* values are inlined at build time. An unset CI variable inlines
// as "" rather than undefined, which `??` does not catch — the empty string then
// reaches `new URL()` and throws "Failed to construct 'URL': Invalid URL" on
// every browser-side request. Treat blank as absent.
function configured(value: string | undefined): string | undefined {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

export function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    return (
      configured((window as unknown as { __HF_API__?: string }).__HF_API__) ??
      configured(process.env.NEXT_PUBLIC_API_URL) ??
      // Production serves the API through Caddy on the storefront's own origin,
      // so same-origin is the right guess when nothing was configured. Local dev
      // sets NEXT_PUBLIC_API_URL explicitly and never reaches this.
      `${window.location.origin}/api/v1`
    );
  }
  return (
    configured(process.env.HAPPYFEET_API_URL) ??
    configured(process.env.NEXT_PUBLIC_API_URL) ??
    "http://localhost:8080/api/v1"
  );
}

const ACCESS_TOKEN_KEY = "hf:access_token";
const REFRESH_TOKEN_KEY = "hf:refresh_token";

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(ACCESS_TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setTokens(access: string, refresh: string) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(ACCESS_TOKEN_KEY, access);
    window.localStorage.setItem(REFRESH_TOKEN_KEY, refresh);
  } catch {
    /* ignore */
  }
}

function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(REFRESH_TOKEN_KEY);
  } catch {
    return null;
  }
}

// One refresh in flight at a time: a page that fires five requests as the
// access token lapses must not spend the refresh token five times over.
let refreshing: Promise<boolean> | null = null;

/**
 * Swaps the refresh token for a fresh pair. Access tokens are short-lived, so a
 * 401 usually means "renew", not "signed out" — only when this also fails is
 * the session really over.
 */
function refreshSession(): Promise<boolean> {
  if (refreshing) return refreshing;
  const refresh = getRefreshToken();
  if (!refresh) return Promise.resolve(false);
  refreshing = (async () => {
    try {
      const res = await fetch(`${getApiBaseUrl()}/auth/refresh`, {
        method: "POST",
        headers: { Accept: "application/json", Authorization: `Bearer ${refresh}` },
        cache: "no-store",
      });
      if (!res.ok) return false;
      const body = (await res.json()) as {
        access_token?: string;
        refresh_token?: string;
        tokens?: { access_token?: string; refresh_token?: string };
      };
      const toks = body.tokens ?? body;
      if (!toks.access_token || !toks.refresh_token) return false;
      setTokens(toks.access_token, toks.refresh_token);
      return true;
    } catch {
      return false;
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

/**
 * The session is over: drop the dead tokens and send the visitor to sign in,
 * carrying where they were so the login page can bring them back.
 */
function sendToLogin() {
  clearTokens();
  if (typeof window === "undefined") return;
  const { pathname, search } = window.location;
  if (pathname.startsWith("/account")) return; // already there; don't loop
  const next = encodeURIComponent(pathname + search);
  window.location.assign(`/account?next=${next}`);
}

export function clearTokens() {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(ACCESS_TOKEN_KEY);
    window.localStorage.removeItem(REFRESH_TOKEN_KEY);
  } catch {
    /* ignore */
  }
}

export type RequestOptions = {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined | null>;
  headers?: Record<string, string>;
  auth?: boolean;
  idempotencyKey?: string;
  signal?: AbortSignal;
  // Treat as soft-failure: return null on network error rather than throwing.
  // Useful for SSR pre-render so missing backend doesn't crash a route.
  soft?: boolean;
  timeoutMs?: number;
};

function buildUrl(base: string, path: string, query?: RequestOptions["query"]) {
  const url = new URL(path.startsWith("http") ? path : `${base}${path}`);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null || v === "") continue;
      url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}

export async function apiFetch<T>(
  path: string,
  opts: RequestOptions = {},
): Promise<T> {
  return request<T>(path, opts, true);
}

async function request<T>(path: string, opts: RequestOptions, mayRetry: boolean): Promise<T> {
  const base = getApiBaseUrl();
  const url = buildUrl(base, path, opts.query);

  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(opts.headers ?? {}),
  };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.idempotencyKey) headers["Idempotency-Key"] = opts.idempotencyKey;
  let sentToken = false;
  if (opts.auth !== false) {
    const tok = getAccessToken();
    if (tok) {
      headers["Authorization"] = `Bearer ${tok}`;
      sentToken = true;
    }
  }

  const controller = new AbortController();
  const timeoutMs = opts.timeoutMs ?? 8000;
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  const signal = opts.signal ?? controller.signal;

  let res: Response;
  try {
    res = await fetch(url, {
      method: opts.method ?? "GET",
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      signal,
      cache: "no-store",
    });
  } catch (err) {
    clearTimeout(timeout);
    if (opts.soft) return null as unknown as T;
    const e: ApiError = {
      code: "NETWORK_ERROR",
      message: err instanceof Error ? err.message : "network error",
      status: 0,
    };
    throw e;
  }
  clearTimeout(timeout);

  // A 401 on a request that carried a token means the session lapsed. Renew it
  // once and replay; if that fails too, the visitor has to sign in again.
  // Requests sent without a token (sign-in itself, public reads) are left
  // alone — their 401 means wrong credentials, not an expired session.
  if (res.status === 401 && sentToken && typeof window !== "undefined") {
    if (mayRetry && (await refreshSession())) {
      return request<T>(path, opts, false);
    }
    sendToLogin();
  }

  if (res.status === 204) return undefined as unknown as T;

  const text = await res.text();
  let json: unknown = null;
  if (text) {
    try {
      json = JSON.parse(text);
    } catch {
      /* ignore */
    }
  }

  if (!res.ok) {
    if (opts.soft) return null as unknown as T;
    const e = (json as { error?: { code?: string; message?: string; request_id?: string } } | null)?.error;
    const err: ApiError = {
      code: e?.code ?? `HTTP_${res.status}`,
      message: e?.message ?? res.statusText,
      status: res.status,
      requestId: e?.request_id,
    };
    throw err;
  }

  return json as T;
}

export function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  // RFC4122-ish fallback
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
