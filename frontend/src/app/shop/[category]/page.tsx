import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { HF_CATEGORIES, HF_PRODUCTS } from "@/lib/catalog";
import { listProductsApi } from "@/lib/api/products";
import { PlpClient } from "./plp-client";
import { getBlurMap } from "@/lib/placeholder";
import { tServer } from "@/lib/i18n/server";

export const dynamic = "force-dynamic";

const CATEGORY_NAME_KEY: Record<string, string> = {
  womens: "nav.women",
  mens: "nav.men",
  kids: "nav.kids",
  sneakers: "home.cat.sneakers.label",
  loafers: "home.cat.loafers.label",
  boots: "home.cat.boots.label",
  formal: "home.cat.formal.label",
  sandals: "home.cat.sneakers.label", // closest existing label; specific key below if you add one
};

const TITLE_BY_CAT: Record<string, string> = {
  all: "plp.title.all",
  sale: "plp.title.sale",
  sneakers: "plp.title.sneakers",
  boots: "plp.title.boots",
  loafers: "plp.title.loafers",
  formal: "plp.title.formal",
  sandals: "plp.title.sandals",
};

export async function generateMetadata({
  params,
}: {
  params: Promise<{ category: string }>;
}): Promise<Metadata> {
  const { category } = await params;
  const t = await tServer();
  const name = nameFor(category, t);
  return {
    title: name,
    description: `${name} — Happy Feet.`,
  };
}

function nameFor(slug: string, t: (k: string) => string): string {
  if (slug === "sale") return t("plp.sale_name");
  if (slug === "all") return t("plp.all_name");
  const key = CATEGORY_NAME_KEY[slug];
  if (key) return t(key);
  const cat = HF_CATEGORIES.find((c) => c.slug === slug);
  return cat?.name ?? slug;
}

export default async function CategoryPage({
  params,
}: {
  params: Promise<{ category: string }>;
}) {
  const { category } = await params;
  const t = await tServer();
  const cat = HF_CATEGORIES.find((c) => c.slug === category);
  const isSpecial = category === "sale" || category === "all";
  if (!cat && !isSpecial) {
    notFound();
  }

  // Pull from the live API (with fallback to static catalog).
  const initial = await listProductsApi({ category, limit: 48 });
  const allBrands = Array.from(new Set(HF_PRODUCTS.map((p) => p.brand))).sort();
  const blurs = await getBlurMap(initial.map((p) => p.image));

  const name = nameFor(category, t);
  const titleKey = TITLE_BY_CAT[category];
  const title = titleKey ? t(titleKey) : `${name}.`;
  const eyebrow = t("plp.eyebrow", { category: name });

  return (
    <PlpClient
      categorySlug={category}
      categoryName={name}
      title={title}
      eyebrow={eyebrow}
      brands={allBrands}
      initial={initial}
      blurs={blurs}
    />
  );
}
