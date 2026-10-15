import type { Category } from "./types";

// Navigation fallbacks only. There is deliberately no product list here: the
// catalogue is whatever the API holds, and an empty shop shows as empty.

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
