import { apiFetch, newIdempotencyKey } from "./client";

export type Payment = {
  id: string;
  order_id: string;
  provider: "mtn_momo" | "orange_money" | "stripe" | "cash_on_delivery";
  status: string;
  amount: number;
  currency: string;
  provider_reference: string | null;
  ussd_string: string | null;
  checkout_url: string | null;
  expires_at: string | null;
  initiated_at: string;
  confirmed_at: string | null;
};

export type InitiatePaymentInput = {
  order_id: string;
  provider: Payment["provider"];
  phone_number?: string;
  return_url?: string;
};

export async function initiatePayment(input: InitiatePaymentInput): Promise<Payment> {
  return apiFetch<Payment>("/payments/initiate", {
    method: "POST",
    body: input,
    idempotencyKey: newIdempotencyKey(),
  });
}

export async function getPaymentStatus(id: string): Promise<Payment> {
  return apiFetch<Payment>(`/payments/${id}/status`);
}
