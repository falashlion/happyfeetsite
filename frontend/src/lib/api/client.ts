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

export function getApiBaseUrl(): string {
  if (typeof window !== "undefined") {
    return (
      (window as unknown as { __HF_API__?: string }).__HF_API__ ??
      process.env.NEXT_PUBLIC_API_URL ??
      "http://localhost:8080/api/v1"
    );
  }
  return (
    process.env.HAPPYFEET_API_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
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
  const base = getApiBaseUrl();
  const url = buildUrl(base, path, opts.query);

  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(opts.headers ?? {}),
  };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.idempotencyKey) headers["Idempotency-Key"] = opts.idempotencyKey;
  if (opts.auth !== false) {
    const tok = getAccessToken();
    if (tok) headers["Authorization"] = `Bearer ${tok}`;
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
