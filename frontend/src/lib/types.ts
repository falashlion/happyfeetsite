export type Currency = "XAF" | "XOF" | "USD" | "EUR" | "GHS" | "UGX" | "NGN";

export type CategorySlug =
  | "womens"
  | "mens"
  | "kids"
  | "sneakers"
  | "loafers"
  | "boots"
  | "formal"
  | "sandals";

export type Category = { slug: CategorySlug | string; name: string };

export type ProductColor = { name: string; hex: string };

export type ProductSku = {
  id: string;
  sizeEu: number;
  color: string;
  colorHex?: string | null;
  stockQty: number;
};

export type Product = {
  id: string;
  slug: string;
  brand: string;
  category: string;
  name: string;
  subtitle: string;
  price: number;
  was: number | null;
  currency: Currency;
  badge: string | null;
  image: string;
  images: string[];
  sizes: number[];
  oosSizes: number[];
  colors: ProductColor[];
  description: string;
  materials: string[];
  rating: number;
  ratingCount: number;
  inStock: boolean;
  // Optional in static catalog, present when fetched from the backend.
  skus?: ProductSku[];
};

export type BagItem = {
  productId: string;
  // Backend SKU UUID — required to sync the cart / place an order on the live API.
  // Optional so locally-mocked bag items still type-check.
  skuId?: string;
  size: number;
  color: ProductColor;
  qty: number;
};

export type WishItem = { productId: string };
