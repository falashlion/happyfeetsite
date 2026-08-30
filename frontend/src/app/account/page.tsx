import type { Metadata } from "next";
import { AccountClient } from "./account-client";
import { tServer } from "@/lib/i18n/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await tServer();
  return { title: t("auth.metadata_title") };
}
// Auth state depends on a client localStorage token + locale cookie — never cache.
export const dynamic = "force-dynamic";

export default function AccountPage() {
  return <AccountClient />;
}
