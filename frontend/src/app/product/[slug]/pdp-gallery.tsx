"use client";

import Image from "next/image";
import { useState } from "react";
import { useT } from "@/lib/i18n/context";

export function PdpGalleryClient({
  images,
  blurs,
  altBase,
}: {
  images: string[];
  blurs: Record<string, string>;
  altBase: string;
}) {
  const { t } = useT();
  const [idx, setIdx] = useState(0);
  const src = images[idx];
  return (
    <div className="pdp-gallery">
      <div className="pdp-main">
        <Image
          src={src}
          alt={altBase}
          fill
          priority
          sizes="(max-width: 900px) 100vw, 640px"
          placeholder="blur"
          blurDataURL={blurs[src]}
        />
      </div>
      {/* A lone thumbnail is just the hero again at 1/8 scale — no affordance,
          only clutter. Show the strip once there is something to switch to. */}
      <div className="pdp-thumbs" hidden={images.length < 2}>
        {images.map((s, i) => (
          <button
            key={s + i}
            className={i === idx ? "active" : ""}
            onClick={() => setIdx(i)}
            aria-label={t("pdp.gallery_image_aria", { n: i + 1 })}
          >
            <Image
              src={s}
              alt=""
              fill
              sizes="120px"
              placeholder="blur"
              blurDataURL={blurs[s]}
            />
          </button>
        ))}
      </div>
    </div>
  );
}
