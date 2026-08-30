"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Search, User, ShoppingBag, Menu, X } from "lucide-react";
import { useCartCount, useCart } from "@/store/cart";
import { useT } from "@/lib/i18n/context";
import { LocaleToggle } from "@/components/locale-toggle";

const LINKS: { slug: string; key: string }[] = [
  { slug: "womens", key: "nav.women" },
  { slug: "mens", key: "nav.men" },
  { slug: "kids", key: "nav.kids" },
  { slug: "sneakers", key: "nav.sneakers" },
  { slug: "boots", key: "nav.boots" },
  { slug: "sale", key: "nav.sale" },
];

export function Header() {
  const [menuOpen, setMenuOpen] = useState(false);
  const path = usePathname();
  const count = useCartCount();
  const hydrated = useCart((s) => s.hydrated);
  const { t } = useT();

  useEffect(() => setMenuOpen(false), [path]);
  useEffect(() => {
    if (menuOpen) {
      const prev = document.body.style.overflow;
      document.body.style.overflow = "hidden";
      return () => {
        document.body.style.overflow = prev;
      };
    }
  }, [menuOpen]);

  const badge = hydrated && count > 0 ? <span className="bag-count">{count}</span> : null;

  return (
    <>
      <div className="promo-strip">
        <span className="gold">{t("promo.shipping_free")}</span> ·{" "}
        {t("promo.delivery_time")} ·{" "}
        <span className="gold">{t("promo.returns")}</span>
      </div>
      <header className="nav">
        <div className="nav-left">
          <button
            className="lg:hidden p-2 -ml-2"
            onClick={() => setMenuOpen(true)}
            aria-label={t("nav.menu")}
          >
            <Menu size={20} strokeWidth={1.5} />
          </button>
          <Link href="/" className="nav-lockup" aria-label={`${t("common.brand")} — home`}>
            <Image
              src="/brand/logo-mark-navy.png"
              alt={t("common.brand")}
              width={40}
              height={40}
              priority
            />
            <span className="name">
              Happy<em>·</em>Feet
            </span>
          </Link>
        </div>

        {/* The six category links plus the locale toggle and three actions only
            fit from 1024px up; below that the drawer takes over. Keep these
            breakpoints in step with the nav rules in globals.css. */}
        <nav className="nav-links hidden lg:flex">
          {LINKS.map((l) => {
            const href = l.slug === "sale" ? "/sale" : `/shop/${l.slug}`;
            const active = path === href;
            return (
              <Link key={l.slug} href={href} className={active ? "active" : ""}>
                {t(l.key)}
              </Link>
            );
          })}
        </nav>

        <div className="nav-actions">
          <div className="hidden lg:flex">
            <LocaleToggle />
          </div>
          <Link href="/shop/sneakers" className="nav-action" aria-label={t("nav.search")}>
            <Search size={18} strokeWidth={1.5} />
            <span>{t("nav.search")}</span>
          </Link>
          <Link href="/account" className="nav-action" aria-label={t("nav.account")}>
            <User size={18} strokeWidth={1.5} />
            <span>{t("nav.account")}</span>
          </Link>
          <Link
            href="/bag"
            className="nav-action"
            aria-label={`${t("nav.bag")}${count ? ` — ${count} ${count === 1 ? t("common.pair_one") : t("common.pair_other")}` : ""}`}
          >
            <ShoppingBag size={18} strokeWidth={1.5} />
            <span>{t("nav.bag")}</span>
            {badge}
          </Link>
        </div>
      </header>

      {menuOpen && (
        <MobileDrawer onClose={() => setMenuOpen(false)} count={count} />
      )}
    </>
  );
}

function MobileDrawer({
  onClose,
  count,
}: {
  onClose: () => void;
  count: number;
}) {
  const { t } = useT();
  return (
    <div
      role="dialog"
      aria-label={t("nav.menu")}
      className="fixed inset-0 z-[1100] bg-[rgba(14,27,58,0.6)]"
      onClick={onClose}
    >
      <aside
        className="absolute inset-y-0 left-0 w-[min(360px,86vw)] bg-[var(--bg-canvas)] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="flex items-center justify-between p-5 border-b border-[var(--border-hair)]">
          <div className="nav-lockup">
            <Image src="/brand/logo-mark-navy.png" alt="HF" width={36} height={36} />
            <span className="name">
              Happy<em>·</em>Feet
            </span>
          </div>
          <button onClick={onClose} aria-label={t("nav.close")} className="p-2 -mr-2">
            <X size={20} strokeWidth={1.5} />
          </button>
        </header>

        <div className="p-5 flex-1 overflow-y-auto">
          <div className="eyebrow eyebrow-muted mb-3">{t("nav.section_shop")}</div>
          <nav className="flex flex-col">
            {LINKS.map((l) => (
              <Link
                key={l.slug}
                href={l.slug === "sale" ? "/sale" : `/shop/${l.slug}`}
                className="py-3 border-b border-[var(--border-hair)] flex justify-between items-center text-[15px]"
              >
                <span>{t(l.key)}</span>
              </Link>
            ))}
          </nav>

          <div className="eyebrow eyebrow-muted mt-7 mb-3">{t("nav.section_account")}</div>
          <nav className="flex flex-col">
            <Link
              href="/account"
              className="py-3 border-b border-[var(--border-hair)] text-[15px]"
            >
              {t("nav.sign_in_register")}
            </Link>
            <Link
              href="/bag"
              className="py-3 border-b border-[var(--border-hair)] flex justify-between items-center text-[15px]"
            >
              <span>{t("nav.your_bag")}</span>
              {count > 0 && <span className="bag-count">{count}</span>}
            </Link>
          </nav>

          <div className="eyebrow eyebrow-muted mt-7 mb-3">{t("nav.language")}</div>
          <LocaleToggle variant="list" />
        </div>
      </aside>
    </div>
  );
}
