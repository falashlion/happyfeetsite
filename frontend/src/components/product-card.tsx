"use client";

import Image from "next/image";
import Link from "next/link";
import { useOptimistic, useTransition } from "react";
import { Heart } from "lucide-react";
import type { Product } from "@/lib/types";
import { formatPrice } from "@/lib/format";
import { useWishlist } from "@/store/wishlist";
import { useT } from "@/lib/i18n/context";

export function ProductCard({
  product,
  priority = false,
  blurDataURL,
}: {
  product: Product;
  priority?: boolean;
  blurDataURL?: string;
}) {
  const { t } = useT();
  const wishlistIds = useWishlist((s) => s.ids);
  const toggle = useWishlist((s) => s.toggle);
  const saved = wishlistIds.includes(product.id);

  const [optimisticSaved, setOptimisticSaved] = useOptimistic(saved);
  const [, startTransition] = useTransition();

  const onWish = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    startTransition(() => {
      setOptimisticSaved(!optimisticSaved);
      toggle(product.id);
    });
  };

  const badgeClass =
    product.badge?.startsWith("−") ? "navy" :
    product.badge === "HOUSE FAVOURITE" ? "gold" : "";

  return (
    <Link href={`/product/${product.slug}`} className="pc" prefetch>
      <div className="pc-img">
        <Image
          src={product.image}
          alt={`${product.brand} ${product.name}`}
          fill
          sizes="(max-width: 760px) 50vw, (max-width: 1200px) 33vw, 240px"
          priority={priority}
          placeholder={blurDataURL ? "blur" : "empty"}
          blurDataURL={blurDataURL}
        />
        {product.badge && (
          <span className={`pc-badge ${badgeClass}`}>{product.badge}</span>
        )}
        <button
          className={`pc-wish ${optimisticSaved ? "saved" : ""}`}
          aria-label={optimisticSaved ? t("pdp.save_aria") : t("pdp.save_aria")}
          aria-pressed={optimisticSaved}
          onClick={onWish}
        >
          <Heart
            size={16}
            strokeWidth={1.5}
            fill={optimisticSaved ? "currentColor" : "none"}
          />
        </button>
      </div>
      <div className="pc-body">
        <span className="pc-brand">{product.brand}</span>
        <h4 className="pc-name">{product.name}</h4>
        <div className="pc-price">
          {formatPrice(product.price, product.currency)}
          {product.was && (
            <span className="pc-was">{formatPrice(product.was, product.currency)}</span>
          )}
        </div>
        <div className="pc-sizes">
          {product.sizes.slice(0, 6).map((s) => (
            <span key={s} className={product.oosSizes.includes(s) ? "oos" : ""}>
              {s}
            </span>
          ))}
        </div>
      </div>
    </Link>
  );
}
