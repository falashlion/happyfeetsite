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
