import "server-only";
import { getPlaiceholder } from "plaiceholder";

const cache = new Map<string, string>();

/**
 * Fetch the remote image and produce a base64 blur-up placeholder.
 * Runs at build time during ISR, then the result is served from CDN.
 * Failures fall back to a flat cream pixel — never throw into a page render.
 */
export async function getBlur(src: string): Promise<string> {
  if (cache.has(src)) return cache.get(src)!;
  try {
    const res = await fetch(src);
    if (!res.ok) throw new Error(`fetch ${res.status}`);
    const buf = Buffer.from(await res.arrayBuffer());
    const { base64 } = await getPlaiceholder(buf, { size: 10 });
    cache.set(src, base64);
    return base64;
  } catch {
    const fallback =
      "data:image/svg+xml;base64," +
      Buffer.from(
        `<svg xmlns="http://www.w3.org/2000/svg" width="4" height="5"><rect width="4" height="5" fill="#F5EFE0"/></svg>`,
      ).toString("base64");
    cache.set(src, fallback);
    return fallback;
  }
}

export async function getBlurMap(srcs: string[]): Promise<Record<string, string>> {
  const entries = await Promise.all(srcs.map(async (s) => [s, await getBlur(s)] as const));
  return Object.fromEntries(entries);
}
