import type { Metadata } from "next";
import { ConfirmationClient } from "./confirmation-client";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("confirmation.metadata_title") };
}
export const dynamic = "force-dynamic";

export default function ConfirmationPage() {
  return <ConfirmationClient />;
}
