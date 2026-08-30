"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { BagItem } from "@/lib/types";

type CartState = {
  items: BagItem[];
  hydrated: boolean;
  setHydrated: (v: boolean) => void;
  add: (item: BagItem) => void;
  remove: (index: number) => void;
  setQty: (index: number, qty: number) => void;
  clear: () => void;
};

export const useCart = create<CartState>()(
  persist(
    (set) => ({
      items: [],
      hydrated: false,
      setHydrated: (v) => set({ hydrated: v }),
      add: (item) =>
        set((s) => {
          const existing = s.items.findIndex(
            (i) =>
              (item.skuId && i.skuId === item.skuId) ||
              (i.productId === item.productId &&
                i.size === item.size &&
                i.color.name === item.color.name),
          );
          if (existing >= 0) {
            const next = s.items.slice();
            next[existing] = {
              ...next[existing],
              skuId: next[existing].skuId ?? item.skuId,
              qty: next[existing].qty + item.qty,
            };
            return { items: next };
          }
          return { items: [...s.items, item] };
        }),
      remove: (index) =>
        set((s) => ({ items: s.items.filter((_, i) => i !== index) })),
      setQty: (index, qty) =>
        set((s) => ({
          items: s.items.map((it, i) =>
            i === index ? { ...it, qty: Math.max(1, qty) } : it,
          ),
        })),
      clear: () => set({ items: [] }),
    }),
    {
      name: "hf:bag",
      storage: createJSONStorage(() => localStorage),
      onRehydrateStorage: () => (state) => state?.setHydrated(true),
    },
  ),
);

export function useCartCount() {
  const items = useCart((s) => s.items);
  return items.reduce((n, i) => n + i.qty, 0);
}
