import type { Metadata } from "next";
import { BagClient } from "./bag-client";
import { HF_PRODUCTS } from "@/lib/catalog";
import { getBlurMap } from "@/lib/placeholder";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("bag.metadata_title") };
}
export const dynamic = "force-dynamic";

export default async function BagPage() {
  // Pre-compute blurs for every product image so any bag item renders crisply.
  const blurs = await getBlurMap(HF_PRODUCTS.map((p) => p.image));
  return <BagClient blurs={blurs} />;
}
