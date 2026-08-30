import type { Metadata } from "next";
import { CheckoutClient } from "./checkout-client";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("checkout.metadata_title") };
}
export const dynamic = "force-dynamic";

export default function CheckoutPage() {
  return <CheckoutClient />;
}
