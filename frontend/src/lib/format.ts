import type { Currency } from "./types";

// Accepts the typed Currency union for our own code paths, and a bare string
// for values that originate from the backend (where currency is just a string).
export function formatPrice(amount: number, currency: Currency | string = "XAF"): string {
  const grouped = Math.round(amount)
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return `${currency} ${grouped}`;
}
