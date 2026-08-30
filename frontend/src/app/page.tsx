import Image from "next/image";
import Link from "next/link";
import { ArrowRight, Truck, RotateCcw, Shield, Sparkles } from "lucide-react";
import { HF_PRODUCTS } from "@/lib/catalog";
import { listProductsApi } from "@/lib/api/products";
import { ProductCard } from "@/components/product-card";
import { SectionHead } from "@/components/section-head";
import { getBlurMap } from "@/lib/placeholder";
import { tServer } from "@/lib/i18n/server";

// Server pages depend on the locale cookie — never cache.
export const dynamic = "force-dynamic";

export default async function HomePage() {
  const t = await tServer();
  const all = await listProductsApi({ limit: 24 });
  const source = all.length > 0 ? all : HF_PRODUCTS;
  const featured = source.slice(0, 4);
  const houseFavs = source.slice(4, 8);
  const editorial = source[1] ?? source[0];
  const heroProduct = source[2] ?? source[0];

  const CATEGORIES = [
    { slug: "sneakers", label: t("home.cat.sneakers.label"), sub: t("home.cat.sneakers.sub"), src: "https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=800&q=80" },
    { slug: "loafers",  label: t("home.cat.loafers.label"),  sub: t("home.cat.loafers.sub"),  src: "https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=800&q=80" },
    { slug: "boots",    label: t("home.cat.boots.label"),    sub: t("home.cat.boots.sub"),    src: "https://images.unsplash.com/photo-1549298916-b41d501d3772?w=800&q=80" },
    { slug: "formal",   label: t("home.cat.formal.label"),   sub: t("home.cat.formal.sub"),   src: "https://images.unsplash.com/photo-1582588678413-dbf45f4823e9?w=800&q=80" },
  ];

  const PROMISES = [
    { Icon: Truck,    title: t("home.promise.truck.title"),    body: t("home.promise.truck.body") },
    { Icon: RotateCcw,title: t("home.promise.returns.title"),  body: t("home.promise.returns.body") },
    { Icon: Shield,   title: t("home.promise.authentic.title"),body: t("home.promise.authentic.body") },
    { Icon: Sparkles, title: t("home.promise.sizing.title"),   body: t("home.promise.sizing.body") },
  ];

  const allImages = [
    heroProduct.image,
    editorial.image,
    ...featured.map((p) => p.image),
    ...houseFavs.map((p) => p.image),
    ...CATEGORIES.map((c) => c.src),
  ];
  const blurs = await getBlurMap(allImages);

  return (
    <>
      <section className="hero">
        <div className="hero-grid">
          <div>
            <div className="eyebrow" style={{ color: "var(--hf-gold-300)" }}>
              {t("home.eyebrow")}
            </div>
            <h1 style={{ marginTop: 18 }}>
              {t("home.h1_l1")}
              <br />
              {t("home.h1_l2")}
            </h1>
            <p className="lede">{t("home.lede")}</p>
            <div className="actions">
              <Link href="/shop/sneakers" className="btn btn-gold btn-lg">
                {t("home.cta_browse")} <ArrowRight size={18} strokeWidth={1.5} />
              </Link>
              <Link
                href={`/product/${heroProduct.slug}`}
                className="btn btn-secondary btn-lg"
                style={{ color: "var(--hf-cream-50)", borderColor: "var(--hf-cream-50)" }}
              >
                {t("home.cta_hero_product")}
              </Link>
            </div>
          </div>
          <div className="figure">
            <div className="photo">
              <Image
                src={heroProduct.image}
                alt={`${heroProduct.brand} ${heroProduct.name}`}
                fill
                priority
                sizes="(max-width: 860px) 100vw, 600px"
                placeholder="blur"
                blurDataURL={blurs[heroProduct.image]}
              />
            </div>
            <div className="seal">
              <div className="top">EST · 2024</div>
              <div className="big">H&#8734;F</div>
              <div className="btm">DOUALA</div>
            </div>
          </div>
        </div>
      </section>

      <div className="container-hf">
        <div className="promises">
          {PROMISES.map(({ Icon, title, body }) => (
            <div className="promise" key={title}>
              <Icon size={22} strokeWidth={1.5} />
              <div>
                <div className="title">{title}</div>
                <div className="body">{body}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      <section className="container-hf" style={{ padding: "88px 0 0" }}>
        <SectionHead
          eyebrow={t("home.section.shopby_eyebrow")}
          title={t("home.section.shopby_title")}
          action={
            <Link href="/shop/sneakers" className="btn btn-ghost">
              {t("home.section.shopby_action")} <ArrowRight size={16} strokeWidth={1.5} />
            </Link>
          }
        />
        <div className="cat-rail">
          {CATEGORIES.map((c) => (
            <Link key={c.slug} href={`/shop/${c.slug}`} className="cat-tile">
              <Image
                src={c.src}
                alt={c.label}
                fill
                sizes="(max-width: 760px) 50vw, 25vw"
                placeholder="blur"
                blurDataURL={blurs[c.src]}
              />
              <span className="sublabel">{c.sub}</span>
              <span className="label">{c.label}</span>
            </Link>
          ))}
        </div>
      </section>

      <section className="container-hf" style={{ padding: "88px 0 0" }}>
        <SectionHead
          eyebrow={t("home.section.new_eyebrow")}
          title={t("home.section.new_title")}
          action={
            <Link href="/shop/sneakers" className="btn btn-ghost">
              {t("home.section.new_action")} <ArrowRight size={16} strokeWidth={1.5} />
            </Link>
          }
        />
        <div className="product-grid">
          {featured.map((p, i) => (
            <ProductCard
              key={p.id}
              product={p}
              priority={i < 2}
              blurDataURL={blurs[p.image]}
            />
          ))}
        </div>
      </section>

      <section className="container-hf">
        <div className="editorial">
          <div className="ed-img">
            <Image
              src={editorial.image}
              alt={`${editorial.brand} ${editorial.name}`}
              fill
              sizes="(max-width: 860px) 100vw, 600px"
              placeholder="blur"
              blurDataURL={blurs[editorial.image]}
            />
          </div>
          <div className="ed-body">
            <div className="eyebrow">{t("home.editorial.eyebrow")}</div>
            <h3>{t("home.editorial.quote")}</h3>
            <p>{t("home.editorial.body")}</p>
            <Link href="/about" className="btn btn-secondary" style={{ marginTop: 24 }}>
              {t("home.editorial.cta")}
            </Link>
          </div>
        </div>
      </section>

      <section className="container-hf" style={{ paddingBottom: 32 }}>
        <SectionHead
          eyebrow={t("home.section.favs_eyebrow")}
          title={t("home.section.favs_title")}
        />
        <div className="product-grid">
          {houseFavs.map((p) => (
            <ProductCard key={p.id} product={p} blurDataURL={blurs[p.image]} />
          ))}
        </div>
      </section>
    </>
  );
}
