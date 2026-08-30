import { apiFetch, setTokens, clearTokens, getAccessToken } from "./client";

export type AuthUser = {
  id: string;
  email: string | null;
  phone: string | null;
  first_name: string;
  last_name: string;
  role: string;
  status: string;
  avatar_url: string | null;
};

type LoginResponse = {
  tokens: {
    access_token: string;
    refresh_token: string;
    token_type: string;
    expires_in: number;
  };
  user: AuthUser;
};

type RegisterResponse = {
  user_id: string;
  message: string;
};

export async function register(input: {
  first_name: string;
  last_name: string;
  email?: string;
  phone?: string;
  password: string;
}): Promise<RegisterResponse> {
  return apiFetch<RegisterResponse>("/auth/register", {
    method: "POST",
    body: input,
    auth: false,
  });
}

export async function login(input: {
  email?: string;
  phone?: string;
  password: string;
}): Promise<LoginResponse> {
  const res = await apiFetch<LoginResponse>("/auth/login", {
    method: "POST",
    body: input,
    auth: false,
  });
  setTokens(res.tokens.access_token, res.tokens.refresh_token);
  return res;
}

export type AuthProviders = {
  google: { enabled: boolean; client_id?: string };
};

/**
 * Which social sign-in providers the API has configured, and the public client
 * ID to initialise them with. Asking the API keeps the client ID in one place
 * (the server's env) instead of duplicated into the web build, where it would
 * silently drift.
 *
 * Soft-fails: a storefront that cannot reach the API should still render the
 * email/password form rather than an error.
 */
export async function getAuthProviders(): Promise<AuthProviders | null> {
  return apiFetch<AuthProviders>("/auth/providers", { auth: false, soft: true });
}

/**
 * Exchanges the Google ID token (the `credential` from Google Identity
 * Services) for our own token pair, creating the account on first use.
 */
export async function loginWithGoogle(idToken: string): Promise<GoogleLoginResponse> {
  const res = await apiFetch<GoogleLoginResponse>("/auth/social/google", {
    method: "POST",
    body: { id_token: idToken },
    auth: false,
  });
  setTokens(res.tokens.access_token, res.tokens.refresh_token);
  return res;
}

type GoogleLoginResponse = LoginResponse & {
  /** True when this sign-in created the account rather than matching one. */
  is_new: boolean;
  provider: "google";
};

export async function logout(): Promise<void> {
  try {
    await apiFetch<void>("/auth/logout", { method: "POST", body: {} });
  } finally {
    clearTokens();
  }
}

export async function me(): Promise<AuthUser | null> {
  if (!getAccessToken()) return null;
  try {
    return await apiFetch<AuthUser>("/users/me");
  } catch {
    return null;
  }
}

export function isLoggedIn(): boolean {
  return getAccessToken() !== null;
}
