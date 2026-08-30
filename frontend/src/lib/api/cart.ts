import { apiFetch } from "./client";

export type ApiCartItem = {
  id: string;
  product_id: string;
  sku_id: string;
  product_name: string;
  primary_image_url: string;
  size_eu: number;
  color: string;
  unit_price: number;
  quantity: number;
  subtotal: number;
  stock_available: number;
};

export type ApiCart = {
  id: string;
  items: ApiCartItem[];
  items_count: number;
  subtotal: number;
  currency: string;
  coupon_code: string | null;
  discount_amount: number;
  total: number;
};

export async function getCart(): Promise<ApiCart> {
  return apiFetch<ApiCart>("/cart");
}

export async function clearCart(): Promise<void> {
  await apiFetch<void>("/cart", { method: "DELETE" });
}

export async function addCartItem(sku_id: string, quantity: number): Promise<ApiCart> {
  return apiFetch<ApiCart>("/cart/items", {
    method: "POST",
    body: { sku_id, quantity },
  });
}

export async function updateCartItem(itemId: string, quantity: number): Promise<ApiCart> {
  return apiFetch<ApiCart>(`/cart/items/${itemId}`, {
    method: "PUT",
    body: { quantity },
  });
}

export async function removeCartItem(itemId: string): Promise<ApiCart> {
  return apiFetch<ApiCart>(`/cart/items/${itemId}`, { method: "DELETE" });
}
