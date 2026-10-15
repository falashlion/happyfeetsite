import type { Metadata } from "next";
import { BagClient } from "./bag-client";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("bag.metadata_title") };
}
export const dynamic = "force-dynamic";

export default async function BagPage() {
  // Bag contents live in the browser, so there is nothing to pre-blur here;
  // items render with an empty placeholder until their image loads.
  return <BagClient blurs={{}} />;
}
