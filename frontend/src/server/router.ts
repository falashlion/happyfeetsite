import { z } from "zod";
import { router, publicProcedure } from "./trpc";
import { HF_PRODUCTS, HF_CATEGORIES, HF_BRANDS } from "@/lib/catalog";
import {
  listProductsApi,
  getProductByIdApi,
  getProductBySlugApi,
  listCategoriesApi,
  listBrandsApi,
} from "@/lib/api/products";

const sortEnum = z.enum(["relevance", "price_asc", "price_desc", "rating"]);

export const appRouter = router({
  products: router({
    list: publicProcedure
      .input(
        z
          .object({
            category: z.string().optional(),
            brand: z.string().optional(),
            sort: sortEnum.optional(),
            limit: z.number().int().min(1).max(100).optional(),
          })
          .optional(),
      )
      .query(({ input }) => listProductsApi(input ?? {})),

    byId: publicProcedure
      .input(z.object({ id: z.string() }))
      .query(({ input }) => getProductByIdApi(input.id)),

    bySlug: publicProcedure
      .input(z.object({ slug: z.string() }))
      .query(({ input }) => getProductBySlugApi(input.slug)),

    // Static "all" — kept for layouts that pre-render an exhaustive list.
    all: publicProcedure.query(() => HF_PRODUCTS),
  }),
  taxonomy: router({
    categories: publicProcedure.query(async () => {
      const cats = await listCategoriesApi();
      // If API returned nothing useful, fall back to the static taxonomy.
      return cats.length > 0 ? cats : HF_CATEGORIES.map((c) => ({ slug: c.slug as string, name: c.name }));
    }),
    brands: publicProcedure.query(async () => {
      const brands = await listBrandsApi();
      return brands.length > 0 ? brands : [...HF_BRANDS];
    }),
  }),
});

export type AppRouter = typeof appRouter;
