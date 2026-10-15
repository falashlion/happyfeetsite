import type { Product, ProductColor, ProductSku, Currency } from "@/lib/types";
import { apiFetch } from "./client";
import { colorHex } from "@/lib/colors";

// Shown when a product has no photo yet, instead of borrowing another
// product's picture.
export const NO_IMAGE = "/brand/packaging-giftbox.png";

// ── Backend wire types ────────────────────────────────────────────────────────

type ApiImage = {
  id: string;
  url_thumbnail?: string;
  url_small?: string;
  url_medium?: string;
  url_large?: string;
  url_original?: string;
  is_primary: boolean;
  sort_order: number;
};

type ApiSku = {
  id: string;
  sku_code: string;
  size_eu: number;
  size_us: number | null;
  size_uk: number | null;
  color: string;
  color_hex: string | null;
  width: string;
  stock_qty: number;
  additional_price: number;
};

type ApiProductDetail = {
  id: string;
  name: string;
  slug: string;
  description: string | null;
  brand: string;
  brand_id: string;
  category: string;
  category_id: string;
  category_slug?: string;
  vendor_id: string;
  vendor_name: string;
  base_price: number;
  final_price: number;
  currency: Currency;
  status: string;
  tags: string[];
  rating_avg: number;
  rating_count: number;
  in_stock: boolean;
  images: ApiImage[];
  skus: ApiSku[];
};

type ApiProductSummary = {
  id: string;
  name: string;
  slug: string;
  brand: string;
  category: string;
  category_slug?: string;
  base_price: number;
  final_price: number;
  currency: Currency;
  rating_avg: number;
  rating_count: number;
  in_stock: boolean;
  primary_image: ApiImage | null;
};

type Envelope<T> = { data: T; meta?: unknown };

// ── Adapters: API shape → frontend Product shape ──────────────────────────────

function bestImage(img: ApiImage | null | undefined): string | null {
  if (!img) return null;
  return (
    img.url_medium ||
    img.url_large ||
    img.url_small ||
    img.url_original ||
    img.url_thumbnail ||
    null
  );
}

function dedupeBy<T, K>(arr: T[], key: (v: T) => K): T[] {
  const seen = new Set<K>();
  const out: T[] = [];
  for (const v of arr) {
    const k = key(v);
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(v);
  }
  return out;
}

// The API sends the category's real slug. Older API builds did not, so derive
// one from the label the same way the server slugifies — never by keyword
// matching, which filed "Women's Shoes" under men's.
function categorySlug(slug: string | undefined, label: string | undefined): string {
  if (slug) return slug;
  return (label ?? "")
    .toLowerCase()
    .replace(/'/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
}

function adaptDetail(p: ApiProductDetail): Product {
  const images = p.images
    .slice()
    .sort((a, b) => Number(b.is_primary) - Number(a.is_primary) || a.sort_order - b.sort_order)
    .map((i) => bestImage(i))
    .filter((x): x is string => Boolean(x));

  const colors: ProductColor[] = dedupeBy(
    p.skus.map((s) => ({ name: s.color, hex: colorHex(s.color, s.color_hex) })),
    (c) => c.name,
  );

  const allSizes = dedupeBy(
    p.skus.map((s) => s.size_eu),
    (n) => n,
  ).sort((a, b) => a - b);

  const oosSizes = allSizes.filter((size) =>
    p.skus
      .filter((s) => s.size_eu === size)
      .every((s) => s.stock_qty <= 0),
  );

  return {
    id: p.id,
    slug: p.slug,
    brand: p.brand,
    category: categorySlug(p.category_slug, p.category),
    categoryName: p.category ?? "",
    name: p.name,
    subtitle: (p.tags && p.tags[0]) ?? p.category ?? "",
    price: Number(p.final_price ?? p.base_price ?? 0),
    was:
      p.final_price && p.base_price && p.final_price < p.base_price
        ? Number(p.base_price)
        : null,
    currency: (p.currency as Currency) ?? "XAF",
    badge: null,
    image: images[0] ?? NO_IMAGE,
    images: images.length ? images : [NO_IMAGE],
    sizes: allSizes,
    oosSizes,
    colors: colors.length ? colors : [{ name: "Default", hex: "#15161B" }],
    description: p.description ?? "",
    materials: p.tags ?? [],
    rating: p.rating_avg ?? 0,
    ratingCount: p.rating_count ?? 0,
    inStock: p.in_stock,
    skus: p.skus.map<ProductSku>((s) => ({
      id: s.id,
      sizeEu: s.size_eu,
      color: s.color,
      colorHex: s.color_hex,
      stockQty: s.stock_qty,
    })),
  };
}

function adaptSummary(p: ApiProductSummary): Product {
  const img = bestImage(p.primary_image) ?? NO_IMAGE;
  return {
    id: p.id,
    slug: p.slug,
    brand: p.brand,
    category: categorySlug(p.category_slug, p.category),
    categoryName: p.category ?? "",
    name: p.name,
    subtitle: "",
    price: Number(p.final_price ?? p.base_price ?? 0),
    was:
      p.final_price && p.base_price && p.final_price < p.base_price
        ? Number(p.base_price)
        : null,
    currency: (p.currency as Currency) ?? "XAF",
    badge: null,
    image: img,
    images: [img],
    sizes: [],
    oosSizes: [],
    colors: [{ name: "Default", hex: "#15161B" }],
    description: "",
    materials: [],
    rating: p.rating_avg ?? 0,
    ratingCount: p.rating_count ?? 0,
    inStock: p.in_stock,
  };
}

// ── Public API ────────────────────────────────────────────────────────────────
//
// No demo fallback: an empty category shows as empty. Filling it with bundled
// sample shoes made every category look stocked with things nobody sells.

export async function listProductsApi(opts: {
  category?: string;
  brand?: string;
  sort?: "relevance" | "price_asc" | "price_desc" | "rating";
  limit?: number;
}): Promise<Product[]> {
  const isSpecial = opts.category === "sale" || opts.category === "all";
  const query: Record<string, string | number | undefined> = {
    limit: opts.limit ?? 24,
    sort_by: opts.sort,
  };
  if (!isSpecial && opts.category) query.category_slug = opts.category;
  if (opts.brand) query.brand_slug = opts.brand.toLowerCase();
  if (opts.category === "sale") query.on_sale = "true" as unknown as string;

  const res = await apiFetch<Envelope<ApiProductSummary[]>>("/products", {
    query,
    auth: false,
    soft: true,
    timeoutMs: 3000,
  });
  return (res?.data ?? []).map(adaptSummary);
}

export async function getProductBySlugApi(slug: string): Promise<Product | null> {
  const res = await apiFetch<ApiProductDetail>(`/products/by-slug/${encodeURIComponent(slug)}`, {
    auth: false,
    soft: true,
    timeoutMs: 3000,
  });
  return res ? adaptDetail(res) : null;
}

export async function getProductByIdApi(id: string): Promise<Product | null> {
  // Old baskets can still hold demo ids ("p007"); they have no product.
  const looksUuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id);
  if (!looksUuid) return null;
  const res = await apiFetch<ApiProductDetail>(`/products/${id}`, {
    auth: false,
    soft: true,
    timeoutMs: 3000,
  });
  return res ? adaptDetail(res) : null;
}

export async function listCategoriesApi(): Promise<
  { slug: string; name: string; productCount: number }[]
> {
  type ApiCat = { id: string; slug: string; name: string; product_count?: number };
  const res = await apiFetch<Envelope<ApiCat[]>>("/categories", {
    auth: false,
    soft: true,
    timeoutMs: 3000,
  });
  if (!res || !res.data) {
    // Fallback to static list
    const { HF_CATEGORIES } = await import("@/lib/catalog");
    return HF_CATEGORIES.map((c) => ({
      slug: c.slug as string,
      name: c.name,
      productCount: 0,
    }));
  }
  return res.data.map((c) => ({
    slug: c.slug,
    name: c.name,
    productCount: c.product_count ?? 0,
  }));
}

export async function listBrandsApi(): Promise<string[]> {
  type ApiBrand = { id: string; slug: string; name: string };
  const res = await apiFetch<Envelope<ApiBrand[]>>("/brands", {
    auth: false,
    soft: true,
    timeoutMs: 3000,
  });
  if (!res || !res.data) {
    const { HF_BRANDS } = await import("@/lib/catalog");
    return [...HF_BRANDS];
  }
  return res.data.map((b) => b.name);
}
