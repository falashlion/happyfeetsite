import type { Category, Product } from "./types";

export const HF_CATEGORIES: Category[] = [
  { slug: "womens", name: "Women's" },
  { slug: "mens", name: "Men's" },
  { slug: "kids", name: "Kids'" },
  { slug: "sneakers", name: "Sneakers" },
  { slug: "loafers", name: "Loafers" },
  { slug: "boots", name: "Boots" },
  { slug: "formal", name: "Formal & dress" },
  { slug: "sandals", name: "Sandals" },
];

export const HF_BRANDS = [
  "Nike", "Adidas", "Puma", "New Balance", "Reebok",
  "Vans", "Converse", "Timberland", "Dr. Martens", "Clarks",
  "ECCO", "Birkenstock", "Skechers", "Asics", "Under Armour",
] as const;

const img = (id: string) =>
  `https://images.unsplash.com/photo-${id}?w=900&q=80&auto=format&fit=crop`;

export const HF_PRODUCTS: Product[] = [
  {
    id: "p001", slug: "air-max-270-onyx",
    brand: "Nike", category: "sneakers",
    name: "Air Max 270", subtitle: "Lifestyle cushioned runner",
    price: 45000, was: null, currency: "XAF", badge: "NEW",
    image: img("1542291026-7eec264c27ff"),
    images: [
      img("1542291026-7eec264c27ff"),
      img("1556906781-9a412961c28c"),
      img("1605348532760-6753d2c43329"),
      img("1551107696-a4b0c5a0d9a2"),
    ],
    sizes: [39, 40, 41, 42, 43, 44, 45],
    oosSizes: [44],
    colors: [
      { name: "Onyx", hex: "#15161B" },
      { name: "Cream", hex: "#F5EFE0" },
      { name: "Navy", hex: "#0E1B3A" },
    ],
    description:
      "A cushioned everyday silhouette built on a visible Max Air heel. Mesh and synthetic upper, foam midsole, rubber outsole tuned for city wear.",
    materials: ["Mesh upper", "Foam midsole", "Rubber outsole"],
    rating: 4.6, ratingCount: 248, inStock: true,
  },
  {
    id: "p002", slug: "clarks-penny-tan",
    brand: "Clarks", category: "loafers",
    name: "Penny Loafer", subtitle: "Hand-stitched tan leather",
    price: 62000, was: null, currency: "XAF", badge: "HOUSE FAVOURITE",
    image: img("1543163521-1bf539c55dd2"),
    images: [
      img("1543163521-1bf539c55dd2"),
      img("1614253429340-98120bd6d753"),
      img("1582588678413-dbf45f4823e9"),
    ],
    sizes: [40, 41, 42, 43, 44, 45], oosSizes: [],
    colors: [
      { name: "Tan", hex: "#8B5A2B" },
      { name: "Oxblood", hex: "#5A2A2A" },
      { name: "Black", hex: "#15161B" },
    ],
    description:
      "Blake-stitched penny loafer in full-grain leather. Leather lining, leather sole with a thin rubber heel cap. Built to walk in for a season, then yours for a decade.",
    materials: ["Full-grain leather", "Blake stitched", "Leather lined"],
    rating: 4.9, ratingCount: 132, inStock: true,
  },
  {
    id: "p003", slug: "dm-1460-smooth-black",
    brand: "Dr. Martens", category: "boots",
    name: "1460 Smooth Boot", subtitle: "Eight-eye, Goodyear-welted",
    price: 38250, was: 45000, currency: "XAF", badge: "−15%",
    image: img("1549298916-b41d501d3772"),
    images: [
      img("1549298916-b41d501d3772"),
      img("1520639888713-7851133b1ed0"),
      img("1542838687-d8d4b34a44e0"),
    ],
    sizes: [39, 40, 41, 42, 43, 44], oosSizes: [40],
    colors: [
      { name: "Black", hex: "#15161B" },
      { name: "Cherry", hex: "#7A1F1F" },
    ],
    description:
      "The original eight-eye boot. Smooth leather, signature yellow welt stitch, AirWair sole. Built to break in, then never quit.",
    materials: ["Smooth leather", "Goodyear welted", "AirWair sole"],
    rating: 4.7, ratingCount: 1820, inStock: true,
  },
  {
    id: "p004", slug: "adidas-samba-og",
    brand: "Adidas", category: "sneakers",
    name: "Samba OG", subtitle: "Indoor terrace classic",
    price: 52000, was: null, currency: "XAF", badge: null,
    image: img("1606107557195-0e29a4b5b4aa"),
    images: [img("1606107557195-0e29a4b5b4aa"), img("1525966222134-fcfa99b8ae77")],
    sizes: [39, 40, 41, 42, 43, 44, 45], oosSizes: [],
    colors: [
      { name: "Cream", hex: "#F5EFE0" },
      { name: "Black", hex: "#15161B" },
    ],
    description:
      "Low-profile leather and gum-rubber sole. A pitch shoe that took a thirty-year detour through the city.",
    materials: ["Leather upper", "Suede toe", "Gum-rubber sole"],
    rating: 4.8, ratingCount: 612, inStock: true,
  },
  {
    id: "p005", slug: "ecco-derby-cocoa",
    brand: "ECCO", category: "formal",
    name: "Lisbon Derby", subtitle: "Cocoa calfskin",
    price: 78000, was: null, currency: "XAF", badge: null,
    image: img("1614253429340-98120bd6d753"),
    images: [img("1614253429340-98120bd6d753"), img("1582588678413-dbf45f4823e9")],
    sizes: [40, 41, 42, 43, 44, 45], oosSizes: [45],
    colors: [
      { name: "Cocoa", hex: "#5C3A21" },
      { name: "Black", hex: "#15161B" },
    ],
    description:
      "A clean three-eyelet derby for the office or the wedding. Padded collar, leather-lined footbed, a sole that holds a polish.",
    materials: ["Calfskin", "Leather lining", "Stitched-down sole"],
    rating: 4.5, ratingCount: 96, inStock: true,
  },
  {
    id: "p006", slug: "birk-arizona-cream",
    brand: "Birkenstock", category: "sandals",
    name: "Arizona Sandal", subtitle: "Two-strap cork footbed",
    price: 36000, was: null, currency: "XAF", badge: "NEW",
    image: img("1603487742131-4160ec999306"),
    images: [img("1603487742131-4160ec999306"), img("1564594985645-4427056e22e2")],
    sizes: [38, 39, 40, 41, 42, 43], oosSizes: [],
    colors: [
      { name: "Cream", hex: "#F5EFE0" },
      { name: "Mocha", hex: "#7A5A3D" },
      { name: "Black", hex: "#15161B" },
    ],
    description:
      "Adjustable two-strap profile on the legendary cork-and-latex footbed. Shapes to your foot within a fortnight.",
    materials: ["Birko-Flor", "Cork-latex footbed", "EVA outsole"],
    rating: 4.6, ratingCount: 480, inStock: true,
  },
  {
    id: "p007", slug: "nb-993-grey",
    brand: "New Balance", category: "sneakers",
    name: "993 Made in USA", subtitle: "ABZORB-cushioned heritage runner",
    price: 92000, was: null, currency: "XAF", badge: null,
    image: img("1539185441755-769473a23570"),
    images: [img("1539185441755-769473a23570")],
    sizes: [40, 41, 42, 43, 44, 45], oosSizes: [42],
    colors: [
      { name: "Grey", hex: "#8C8E96" },
      { name: "Navy", hex: "#0E1B3A" },
    ],
    description:
      "The dad-shoe that earned the title. Pigskin and mesh, ABZORB cushioning, hand-built in Skowhegan, Maine.",
    materials: ["Pigskin & mesh", "ABZORB midsole"],
    rating: 4.9, ratingCount: 312, inStock: true,
  },
  {
    id: "p008", slug: "vans-old-skool-mono",
    brand: "Vans", category: "sneakers",
    name: "Old Skool Mono", subtitle: "Canvas-suede skate classic",
    price: 32000, was: null, currency: "XAF", badge: null,
    image: img("1525966222134-fcfa99b8ae77"),
    images: [img("1525966222134-fcfa99b8ae77")],
    sizes: [38, 39, 40, 41, 42, 43, 44], oosSizes: [],
    colors: [
      { name: "Black", hex: "#15161B" },
      { name: "Cream", hex: "#F5EFE0" },
    ],
    description:
      "The first Vans to wear the jazz stripe. Canvas-and-suede upper, padded collar, waffle outsole.",
    materials: ["Canvas & suede", "Waffle rubber outsole"],
    rating: 4.7, ratingCount: 904, inStock: true,
  },
];

export function getProductBySlug(slug: string): Product | undefined {
  return HF_PRODUCTS.find((p) => p.slug === slug);
}

export function getProductById(id: string): Product | undefined {
  return HF_PRODUCTS.find((p) => p.id === id);
}

export function listProducts(opts: { category?: string; brand?: string; sort?: string } = {}): Product[] {
  let list = HF_PRODUCTS.slice();
  if (opts.category && opts.category !== "all") {
    if (opts.category === "sale") {
      list = list.filter((p) => p.was != null);
    } else if (["sneakers", "boots", "loafers", "sandals", "formal"].includes(opts.category)) {
      list = list.filter((p) => p.category === opts.category);
    }
  }
  if (opts.brand) list = list.filter((p) => p.brand === opts.brand);
  switch (opts.sort) {
    case "price_asc":
      list.sort((a, b) => a.price - b.price);
      break;
    case "price_desc":
      list.sort((a, b) => b.price - a.price);
      break;
    case "rating":
      list.sort((a, b) => b.rating - a.rating);
      break;
  }
  return list;
}
