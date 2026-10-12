// Admin catalogue operations. Every call here requires an admin-role token;
// the API answers 403 otherwise, so the UI gates on role purely for ergonomics
// and never for security.

import { apiFetch } from "./client";

export type UploadTicket = {
  upload_id: string;
  upload_url: string;
  /** Replayed verbatim alongside the file — the signature covers them. */
  fields: Record<string, string>;
  expires_at: string;
  max_size_bytes: number;
};

export type ProductImage = {
  id: string;
  url_original: string;
  url_large: string;
  url_medium: string;
  url_small: string;
  url_thumbnail: string;
  is_primary: boolean;
  sort_order: number;
};

export type ProductPatch = {
  name?: string;
  description?: string;
  base_price?: number;
  currency?: string;
  status?: "draft" | "active" | "archived";
  category_id?: string;
  brand_id?: string;
  tags?: string[];
};

export async function updateProduct(id: string, patch: ProductPatch) {
  return apiFetch<Record<string, unknown>>(`/admin/products/${id}`, {
    method: "PATCH",
    body: patch,
  });
}

export async function requestUploadTicket(
  purpose: "product_image" | "avatar" | "review_image",
  file: File,
): Promise<UploadTicket> {
  return apiFetch<UploadTicket>("/media/upload", {
    method: "POST",
    body: {
      purpose,
      content_type: file.type,
      size_bytes: file.size,
    },
  });
}

/**
 * Uploads straight to Cloudinary using a server-signed ticket. The file never
 * passes through our API, which keeps large images off the origin entirely.
 * Returns Cloudinary's public ID, which is all the backend needs to derive
 * every rendition.
 */
export async function uploadToCloudinary(ticket: UploadTicket, file: File): Promise<string> {
  const form = new FormData();
  form.append("file", file);
  for (const [k, v] of Object.entries(ticket.fields)) form.append(k, v);

  const res = await fetch(ticket.upload_url, { method: "POST", body: form });
  if (!res.ok) {
    // Cloudinary reports the real reason in its body; surfacing it saves a
    // round of guessing when a signature or folder is wrong.
    let detail = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { error?: { message?: string } };
      if (body?.error?.message) detail = body.error.message;
    } catch {
      /* non-JSON error body — the status is all we have */
    }
    throw new Error(`Cloudinary upload failed: ${detail}`);
  }

  const body = (await res.json()) as { public_id?: string };
  if (!body.public_id) throw new Error("Cloudinary upload returned no public_id");
  return body.public_id;
}

export async function attachImage(
  productId: string,
  input: {
    public_id: string;
    format?: string;
    size_bytes?: number;
    width?: number;
    height?: number;
    is_primary?: boolean;
    sort_order?: number;
  },
): Promise<ProductImage> {
  return apiFetch<ProductImage>(`/admin/products/${productId}/images`, {
    method: "POST",
    body: input,
  });
}

export async function detachImage(productId: string, imageId: string): Promise<void> {
  await apiFetch<void>(`/admin/products/${productId}/images/${imageId}`, {
    method: "DELETE",
  });
}

/** The whole upload path in one call: sign, upload, attach. */
export async function uploadProductImage(
  productId: string,
  file: File,
  opts: { isPrimary?: boolean; sortOrder?: number } = {},
): Promise<ProductImage> {
  const ticket = await requestUploadTicket("product_image", file);
  const publicId = await uploadToCloudinary(ticket, file);
  return attachImage(productId, {
    public_id: publicId,
    format: file.type.replace("image/", ""),
    size_bytes: file.size,
    is_primary: opts.isPrimary ?? false,
    sort_order: opts.sortOrder ?? 0,
  });
}

// ── catalogue reads for the console ───────────────────────────────────────────

export type AdminProduct = {
  id: string;
  name: string;
  slug: string;
  brand: string;
  category: string;
  base_price: number;
  currency: string;
  in_stock: boolean;
  status: "draft" | "active" | "archived";
  primary_image: { url_medium?: string; url_thumbnail?: string } | null;
};

type Envelope<T> = { data: T };

export type ProductFilters = {
  /** Name search. */
  q?: string;
  /** "draft" | "active" | "archived", or "all". */
  status?: string;
  category_id?: string;
};

/**
 * Lists products for the console.
 *
 * Uses /admin/products, not the public /products: the public list is
 * active-only, and a product is created as a draft — it would be invisible the
 * moment you made it. It is also not listProductsApi, which falls back to the
 * bundled demo catalogue whose ids ("p007") have no database row.
 */
export async function listAdminProducts(filters: ProductFilters = {}): Promise<AdminProduct[]> {
  const res = await apiFetch<Envelope<AdminProduct[]>>("/admin/products", {
    query: {
      ...(filters.q ? { q: filters.q } : {}),
      ...(filters.status && filters.status !== "all" ? { status: filters.status } : {}),
      ...(filters.category_id ? { category_id: filters.category_id } : {}),
    },
  });
  return res?.data ?? [];
}

export type Category = { id: string; name: string; slug: string };
export type Brand = { id: string; name: string; slug: string };

export async function listCategories(): Promise<Category[]> {
  const res = await apiFetch<Envelope<Category[]>>("/categories", { auth: false });
  return res?.data ?? [];
}

export async function listBrands(): Promise<Brand[]> {
  const res = await apiFetch<Envelope<Brand[]>>("/brands", { auth: false });
  return res?.data ?? [];
}

export async function createCategory(name: string, parentId?: string) {
  return apiFetch<Category>("/admin/categories", {
    method: "POST",
    body: { name, ...(parentId ? { parent_id: parentId } : {}) },
  });
}

export type NewProduct = {
  name: string;
  category_id: string;
  brand_id: string;
  description: string;
  base_price: number;
  currency: string;
  tags?: string[];
  /** EU sizes to stock. Without a SKU the product cannot be added to a basket. */
  sizes?: number[];
  color?: string;
  stock?: number;
  /** Publish straight away; the column otherwise defaults to draft. */
  status?: "draft" | "active";
};

export async function createProduct(input: NewProduct): Promise<{ id: string }> {
  return apiFetch<{ id: string }>("/admin/products", { method: "POST", body: input });
}

// ── discount events ───────────────────────────────────────────────────────────

export type Promotion = {
  id: string;
  code: string | null;
  name: string;
  discount_type: "percentage" | "fixed_amount" | "free_shipping" | "buy_one_get_one";
  discount_value: number;
  min_order_amount: number | null;
  max_uses: number | null;
  uses_count: number;
  applicable_to: string;
  starts_at: string;
  expires_at: string | null;
  is_active: boolean;
  /** Server-computed "would this apply right now" — is_active alone doesn't say. */
  live: boolean;
};

export async function listPromotions(): Promise<Promotion[]> {
  const res = await apiFetch<Envelope<Promotion[]>>("/admin/promotions");
  return res?.data ?? [];
}

export async function createPromotion(input: {
  name: string;
  code?: string;
  discount_type: Promotion["discount_type"];
  discount_value: number;
  min_order_amount?: number;
  max_uses?: number;
  starts_at?: string;
  expires_at?: string;
}) {
  return apiFetch<Promotion>("/admin/promotions", { method: "POST", body: input });
}

export async function setPromotionActive(id: string, isActive: boolean) {
  await apiFetch<void>(`/admin/promotions/${id}/active`, {
    method: "PATCH",
    body: { is_active: isActive },
  });
}

// ── announcements ─────────────────────────────────────────────────────────────

export async function broadcastNotification(input: {
  title: string;
  body: string;
  action_url?: string;
  type?: string;
}) {
  return apiFetch<{ recipients: number }>("/admin/notifications/broadcast", {
    method: "POST",
    body: input,
  });
}

// ── fulfilment queue ──────────────────────────────────────────────────────────

/** Every status the order table can hold. */
export type OrderStatus =
  | "PLACED" | "PAID" | "PROCESSING" | "SHIPPED" | "DELIVERED"
  | "CANCELLED" | "RETURN_REQUESTED" | "RETURNED" | "REFUNDED";

export type AdminOrderItem = {
  id: string;
  product_name: string;
  primary_image_url: string;
  size_eu: number;
  color: string;
  quantity: number;
  unit_price: number;
  subtotal: number;
};

/** The JSONB address snapshot taken when the order was placed. */
export type OrderAddress = {
  recipient_name?: string;
  recipient_phone?: string;
  street?: string;
  city?: string;
  state?: string;
  postal_code?: string;
  country?: string;
};

export type OrderStatusEvent = {
  from_status: string | null;
  to_status: string;
  actor_name: string;
  reason: string | null;
  created_at: string;
};

export type AdminOrder = {
  id: string;
  order_number: string;
  status: OrderStatus;
  items: AdminOrderItem[];
  customer: { name: string; email: string; phone: string };
  delivery_address: OrderAddress;
  delivery_method: string;
  delivery_fee: number;
  subtotal: number;
  discount_amount: number;
  total_amount: number;
  currency: string;
  payment_method: string;
  placed_at: string;
  shipped_at: string | null;
  delivered_at: string | null;
  /**
   * The moves the server will accept from here. Rendering anything else would
   * be offering a button guaranteed to 409 — the same map enforces both ends.
   */
  next_statuses: OrderStatus[];
  /** Present on the detail endpoint only. */
  history?: OrderStatusEvent[];
};

export type OrderFilters = {
  /** A single status, or "all". */
  status?: string;
  /** Order number, customer name, email or phone. */
  q?: string;
  cursor?: string;
};

export type OrderPage = { orders: AdminOrder[]; cursor: string | null };

export async function listAdminOrders(filters: OrderFilters = {}): Promise<OrderPage> {
  const res = await apiFetch<{ data: AdminOrder[]; meta?: { cursor: string | null } }>(
    "/admin/orders",
    {
      query: {
        ...(filters.status && filters.status !== "all" ? { status: filters.status } : {}),
        ...(filters.q ? { q: filters.q } : {}),
        ...(filters.cursor ? { cursor: filters.cursor } : {}),
      },
    },
  );
  return { orders: res?.data ?? [], cursor: res?.meta?.cursor ?? null };
}

export async function getAdminOrder(id: string): Promise<AdminOrder> {
  return apiFetch<AdminOrder>(`/admin/orders/${id}`);
}

export type StatusUpdate = AdminOrder & {
  /** False when the order was already in that status — nothing was written. */
  changed: boolean;
  /**
   * A wa.me link carrying the same thank-you the delivery email sends, so it
   * can be forwarded by hand. Empty when we hold no phone number, or when the
   * change was not a delivery.
   */
  whatsapp_forward_url: string;
};

export async function updateOrderStatus(
  id: string,
  status: OrderStatus,
  reason?: string,
): Promise<StatusUpdate> {
  return apiFetch<StatusUpdate>(`/admin/orders/${id}/status`, {
    method: "PATCH",
    body: { status, ...(reason ? { reason } : {}) },
  });
}
