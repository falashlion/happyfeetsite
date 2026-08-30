/**
 * Store contact details.
 *
 * Single source of truth for the storefront side of the merchant's identity —
 * the API keeps its own copy under STORE_* env vars for email. Keep the two in
 * step: the WhatsApp number a customer taps on the site and the one printed in
 * their confirmation email must be the same number.
 *
 * NEXT_PUBLIC_* is required because these values are read in client components.
 */

/** wa.me accepts digits only: no "+", spaces, dashes or parentheses. */
function waDigits(raw: string): string {
  return raw.replace(/\D/g, "");
}

export const STORE = {
  name: process.env.NEXT_PUBLIC_STORE_NAME ?? "Happy Feet",
  whatsappNumber: waDigits(
    process.env.NEXT_PUBLIC_STORE_WHATSAPP_NUMBER ?? "237612345678",
  ),
  whatsappDisplay:
    process.env.NEXT_PUBLIC_STORE_WHATSAPP_DISPLAY ?? "+237 6 12 34 56 78",
  supportEmail: process.env.NEXT_PUBLIC_STORE_SUPPORT_EMAIL ?? "care@happyfeet.cm",
  address: process.env.NEXT_PUBLIC_STORE_ADDRESS ?? "Bonapriso, Douala, Cameroon",
} as const;
