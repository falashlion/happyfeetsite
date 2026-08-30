"use client";

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { AppliedCoupon } from "@/lib/coupons";

type CouponState = {
  applied: AppliedCoupon | null;
  set: (c: AppliedCoupon | null) => void;
};

export const useCoupon = create<CouponState>()(
  persist(
    (set) => ({
      applied: null,
      set: (c) => set({ applied: c }),
    }),
    {
      name: "hf:coupon",
      storage: createJSONStorage(() => localStorage),
    },
  ),
);
