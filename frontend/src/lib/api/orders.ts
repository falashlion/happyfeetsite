import { apiFetch, newIdempotencyKey } from "./client";

export type OrderItem = {
  id: string;
  product_id: string;
  sku_id: string;
  product_name: string;
  primary_image_url: string;
  size_eu: number;
  color: string;
  quantity: number;
  unit_price: number;
  subtotal: number;
};

export type Order = {
  id: string;
  order_number: string;
  status: string;
  items: OrderItem[];
  delivery_method: string;
  delivery_fee: number;
  subtotal: number;
  discount_amount: number;
  total_amount: number;
  currency: string;
  payment_method: string;
  placed_at: string;
  updated_at: string;
};

export type CreateOrderInput = {
  delivery_address_id: string;
  delivery_method: "STANDARD" | "EXPRESS" | "SAME_DAY";
  payment_method: "mtn_momo" | "orange_money" | "stripe" | "cash_on_delivery";
};

export async function createOrder(input: CreateOrderInput): Promise<{ order: Order; payment: unknown }> {
  return apiFetch<{ order: Order; payment: unknown }>("/orders", {
    method: "POST",
    body: input,
    idempotencyKey: newIdempotencyKey(),
  });
}

export async function listOrders(status?: string): Promise<{ data: Order[]; meta: unknown }> {
  return apiFetch<{ data: Order[]; meta: unknown }>("/orders", {
    query: { status },
  });
}

export async function getOrder(id: string): Promise<Order> {
  return apiFetch<Order>(`/orders/${id}`);
}
