"use client";

// Staff console: catalogue, categories, discount events, announcements.
// Deliberately plain — this is an internal tool, not storefront surface.
//
// The role check is ergonomics only. Every endpoint it calls is admin-scoped
// server-side and answers 403 regardless of what this chooses to render.

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { me, type AuthUser } from "@/lib/api/auth";
import {
  listAdminProducts, listCategories, listBrands, createCategory, createProduct,
  updateProduct, uploadProductImage, listPromotions, createPromotion,
  setPromotionActive, broadcastNotification,
  type AdminProduct, type Category, type Brand, type Promotion, type ProductPatch,
  type ProductFilters,
} from "@/lib/api/admin";

type Tab = "catalogue" | "new" | "categories" | "discounts" | "announce";
type Msg = { kind: "ok" | "error"; text: string } | null;

const TABS: { id: Tab; label: string }[] = [
  { id: "catalogue", label: "Catalogue" },
  { id: "new", label: "New product" },
  { id: "categories", label: "Categories" },
  { id: "discounts", label: "Discounts" },
  { id: "announce", label: "Announce" },
];

function errText(e: unknown, fallback = "Something went wrong.") {
  return e instanceof Error && e.message ? e.message : fallback;
}

export default function AdminPage() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);
  const [tab, setTab] = useState<Tab>("catalogue");

  useEffect(() => {
    let cancelled = false;
    me().then((u) => {
      if (!cancelled) { setUser(u); setChecking(false); }
    });
    return () => { cancelled = true; };
  }, []);

  if (checking) return <Shell><p className="text-sm text-neutral-500">Checking your session…</p></Shell>;

  if (!user) {
    return (
      <Shell>
        <p className="text-sm">
          You need to <a href="/account" className="underline">sign in</a> to use the console.
        </p>
      </Shell>
    );
  }

  if (user.role !== "admin") {
    return (
      <Shell>
        <p className="text-sm">
          Signed in as <strong>{user.email ?? user.phone}</strong> with the{" "}
          <code className="rounded bg-neutral-100 px-1">{user.role}</code> role. The console needs{" "}
          <code className="rounded bg-neutral-100 px-1">admin</code>.
        </p>
        <p className="mt-2 text-sm text-neutral-500">
          If your role changed recently, sign out and back in — the role travels in your access token.
        </p>
      </Shell>
    );
  }

  return (
    <Shell>
      <nav className="mb-8 flex flex-wrap gap-1 border-b border-neutral-200">
        {TABS.map((tb) => (
          <button
            key={tb.id}
            type="button"
            onClick={() => setTab(tb.id)}
            aria-current={tab === tb.id}
            className={`-mb-px border-b-2 px-4 py-2 text-sm transition ${
              tab === tb.id
                ? "border-neutral-900 font-medium text-neutral-900"
                : "border-transparent text-neutral-500 hover:text-neutral-800"
            }`}
          >
            {tb.label}
          </button>
        ))}
      </nav>

      {tab === "catalogue" && <Catalogue />}
      {tab === "new" && <NewProductForm onCreated={() => setTab("catalogue")} />}
      {tab === "categories" && <Categories />}
      {tab === "discounts" && <Discounts />}
      {tab === "announce" && <Announce />}
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto max-w-5xl px-4 py-10">
      <h1 className="mb-6 text-2xl font-semibold">Catalogue console</h1>
      {children}
    </main>
  );
}

function Note({ msg }: { msg: Msg }) {
  if (!msg) return null;
  return (
    <p
      role="status"
      className={`mt-4 rounded px-3 py-2 text-sm ${
        msg.kind === "ok" ? "bg-green-50 text-green-800" : "bg-red-50 text-red-800"
      }`}
    >
      {msg.text}
    </p>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
        {label}
      </span>
      {children}
    </label>
  );
}

// The sizes and colours a shoe shop actually stocks. "Other" stays available
// because no fixed list survives contact with a real catalogue.
const EU_SIZES = [35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48];
const COLORS = [
  "Black", "White", "Brown", "Tan", "Navy", "Grey",
  "Beige", "Red", "Green", "Blue", "Gold", "Silver",
];

const input = "w-full rounded border border-neutral-300 px-3 py-2 text-sm";
const primary = "rounded bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50";

// ── Catalogue ─────────────────────────────────────────────────────────────────

function Catalogue() {
  const [products, setProducts] = useState<AdminProduct[]>([]);
  const [selected, setSelected] = useState<AdminProduct | null>(null);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState<Msg>(null);
  const [filters, setFilters] = useState<ProductFilters>({ status: "all", q: "" });
  const [cats, setCats] = useState<Category[]>([]);

  useEffect(() => { void listCategories().then(setCats).catch(() => {}); }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const list = await listAdminProducts(filters);
      setProducts(list);
      setSelected((cur) => (cur ? (list.find((p) => p.id === cur.id) ?? null) : null));
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not load products.") });
    } finally {
      setLoading(false);
    }
  }, [filters]);

  // Debounced so typing in the search box is not one request per keystroke.
  useEffect(() => {
    const id = setTimeout(() => { void reload(); }, 250);
    return () => clearTimeout(id);
  }, [reload]);

  const filterBar = (
    <div className="mb-5 flex flex-wrap items-end gap-3">
      <label className="flex-1 min-w-[12rem]">
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Search
        </span>
        <input
          value={filters.q ?? ""}
          onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value }))}
          placeholder="Product name"
          className={input}
        />
      </label>
      <label>
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Status
        </span>
        <select
          value={filters.status ?? "all"}
          onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))}
          className={input}
        >
          <option value="all">All</option>
          <option value="draft">Draft</option>
          <option value="active">Active</option>
          <option value="archived">Archived</option>
        </select>
      </label>
      <label>
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Category
        </span>
        <select
          value={filters.category_id ?? ""}
          onChange={(e) => setFilters((f) => ({ ...f, category_id: e.target.value || undefined }))}
          className={input}
        >
          <option value="">All</option>
          {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </label>
    </div>
  );

  if (loading && products.length === 0) {
    return <div>{filterBar}<p className="text-sm text-neutral-500">Loading catalogue…</p></div>;
  }

  if (products.length === 0) {
    const filtered = Boolean(filters.q) || (filters.status && filters.status !== "all") || filters.category_id;
    if (filtered) {
      return <div>{filterBar}<p className="text-sm text-neutral-500">No products match those filters.</p></div>;
    }
    return (
      <div>
        {filterBar}
        <p className="text-sm">
          Your catalogue is empty. Anything you see on the storefront right now is demo content
          bundled with the site, not real products.
        </p>
        <p className="mt-2 text-sm text-neutral-500">
          Use <strong>New product</strong> to add your first one.
        </p>
        <Note msg={msg} />
      </div>
    );
  }

  return (
    <div>
      {filterBar}
      <div className="grid gap-8 md:grid-cols-[18rem_1fr]">
      <nav aria-label="Products" className="md:border-r md:border-neutral-200 md:pr-6">
        <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Products ({products.length})
        </h2>
        <ul className="max-h-[70vh] space-y-1 overflow-y-auto">
          {products.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                onClick={() => setSelected(p)}
                aria-current={selected?.id === p.id}
                className={`w-full rounded px-3 py-2 text-left text-sm transition ${
                  selected?.id === p.id ? "bg-neutral-900 text-white" : "hover:bg-neutral-100"
                }`}
              >
                <span className="block truncate">{p.name}</span>
                <span className="block truncate text-xs opacity-60">
                  {p.brand}
                  {p.status !== "active" && ` · ${p.status}`}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </nav>

      {selected ? (
        <Editor key={selected.id} product={selected} onSaved={reload} />
      ) : (
        <p className="text-sm text-neutral-500">Select a product to edit it.</p>
      )}
      </div>
    </div>
  );
}

function Editor({ product, onSaved }: { product: AdminProduct; onSaved: () => Promise<void> }) {
  const [name, setName] = useState(product.name);
  const [price, setPrice] = useState(String(product.base_price));
  const [status, setStatus] = useState<"draft" | "active" | "archived">("active");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  const images = product.primary_image
    ? [product.primary_image.url_medium ?? product.primary_image.url_thumbnail ?? ""]
    : [];

  async function save() {
    setBusy(true); setMsg(null);
    // Send only what changed: absent fields are left untouched server-side, so
    // a concurrent edit to a field we did not alter survives.
    const patch: ProductPatch = { status };
    if (name !== product.name) patch.name = name;
    const parsed = Number(price);
    if (Number.isFinite(parsed) && parsed !== product.base_price) patch.base_price = parsed;

    try {
      await updateProduct(product.id, patch);
      await onSaved();
      setMsg({ kind: "ok", text: "Saved." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Save failed.") });
    } finally { setBusy(false); }
  }

  async function upload(file: File) {
    setBusy(true); setMsg(null);
    try {
      await uploadProductImage(product.id, file, { isPrimary: images.length === 0 });
      await onSaved();
      setMsg({ kind: "ok", text: "Image uploaded." });
      if (fileInput.current) fileInput.current.value = "";
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Upload failed.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="space-y-6">
      <Field label="Name">
        <input value={name} onChange={(e) => setName(e.target.value)} className={input} />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={`Price (${product.currency})`}>
          <input value={price} onChange={(e) => setPrice(e.target.value)} inputMode="decimal" className={input} />
        </Field>
        <Field label="Status">
          <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)} className={input}>
            <option value="draft">Draft</option>
            <option value="active">Active</option>
            <option value="archived">Archived</option>
          </select>
        </Field>
      </div>

      <button type="button" onClick={save} disabled={busy} className={primary}>
        {busy ? "Working…" : "Save changes"}
      </button>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Images
        </h3>

        <div className="mb-4 flex flex-wrap gap-3">
          {images.filter(Boolean).map((src) => (
            <Image key={src} src={src} alt="" width={96} height={96}
              className="h-24 w-24 rounded border border-neutral-200 object-cover" />
          ))}
          {images.length === 0 && <p className="text-sm text-neutral-500">No images yet.</p>}
        </div>

        {/* A bare file input reads as nothing at all; this is the primary action
            on this panel, so it gets a real button. */}
        <button
          type="button"
          disabled={busy}
          onClick={() => fileInput.current?.click()}
          className="rounded border-2 border-dashed border-neutral-400 px-5 py-3 text-sm font-medium text-neutral-800 transition hover:border-neutral-900 hover:bg-neutral-50 disabled:opacity-50"
        >
          {busy ? "Uploading…" : images.length === 0 ? "+ Upload an image" : "+ Upload another image"}
        </button>
        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          className="sr-only"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) void upload(f);
          }}
        />
        <p className="mt-2 text-xs text-neutral-500">
          JPEG, PNG or WebP, up to 10&nbsp;MB. Uploads go straight to Cloudinary.
        </p>
      </div>

      <Note msg={msg} />
    </section>
  );
}

// ── New product ───────────────────────────────────────────────────────────────

function NewProductForm({ onCreated }: { onCreated: () => void }) {
  const [cats, setCats] = useState<Category[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [form, setForm] = useState({
    name: "", description: "", base_price: "", currency: "XAF",
    category_id: "", brand_id: "", stock: "10", publish: true,
  });
  const [sizes, setSizes] = useState<number[]>([39, 40, 41, 42, 43, 44]);
  const [otherSize, setOtherSize] = useState("");
  const [color, setColor] = useState("Black");
  const [otherColor, setOtherColor] = useState("");
  // Photos are staged here and uploaded after the product exists — an image
  // needs a product id to attach to, so it cannot go up with the form itself.
  const [photos, setPhotos] = useState<File[]>([]);
  const [busy, setBusy] = useState(false);
  const [step, setStep] = useState("");
  const [msg, setMsg] = useState<Msg>(null);
  const photoInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void (async () => {
      const [c, b] = await Promise.all([listCategories(), listBrands()]);
      setCats(c); setBrands(b);
      setForm((f) => ({
        ...f,
        category_id: f.category_id || c[0]?.id || "",
        brand_id: f.brand_id || b[0]?.id || "",
      }));
    })();
  }, []);

  async function submit() {
    setBusy(true); setMsg(null);
    const price = Number(form.base_price);
    if (!Number.isFinite(price) || price < 0) {
      setMsg({ kind: "error", text: "Enter a valid price." });
      setBusy(false);
      return;
    }
    const chosenColor = (color === "__other" ? otherColor : color).trim();
    if (sizes.length === 0) {
      setMsg({ kind: "error", text: "Pick at least one size — a product with none cannot be bought." });
      setBusy(false);
      return;
    }

    try {
      setStep("Creating product…");
      const created = await createProduct({
        name: form.name.trim(),
        description: form.description.trim(),
        base_price: price,
        currency: form.currency,
        category_id: form.category_id,
        brand_id: form.brand_id,
        sizes: [...sizes].sort((a, b) => a - b),
        color: chosenColor || undefined,
        stock: Number(form.stock) || 0,
        status: form.publish ? "active" : "draft",
      });

      // Upload sequentially: the first photo becomes the primary one, and
      // ordering matters more here than shaving a second off.
      let uploaded = 0;
      for (const [i, file] of photos.entries()) {
        setStep(`Uploading photo ${i + 1} of ${photos.length}…`);
        try {
          await uploadProductImage(created.id, file, { isPrimary: i === 0, sortOrder: i });
          uploaded += 1;
        } catch (e) {
          // A failed photo must not discard a product that was created fine.
          setMsg({
            kind: "error",
            text: `Product created, but photo ${i + 1} failed: ${errText(e)}`,
          });
        }
      }

      if (uploaded === photos.length) {
        setMsg({
          kind: "ok",
          text: photos.length
            ? `Product created with ${uploaded} photo${uploaded === 1 ? "" : "s"}.`
            : "Product created.",
        });
      }
      setForm((f) => ({ ...f, name: "", description: "", base_price: "" }));
      setPhotos([]);
      if (photoInput.current) photoInput.current.value = "";
      onCreated();
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not create the product.") });
    } finally { setBusy(false); setStep(""); }
  }

  const ready = form.name.trim() && form.description.trim() && form.base_price &&
    form.category_id && form.brand_id;

  return (
    <section className="max-w-2xl space-y-4">
      <Field label="Name">
        <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={input} />
      </Field>

      <Field label="Description">
        <textarea rows={4} value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })} className={input} />
      </Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Price">
          <input value={form.base_price} inputMode="decimal"
            onChange={(e) => setForm({ ...form, base_price: e.target.value })} className={input} />
        </Field>
        <Field label="Currency">
          <select value={form.currency} onChange={(e) => setForm({ ...form, currency: e.target.value })} className={input}>
            {["XAF", "XOF", "USD", "EUR", "GHS", "NGN", "UGX"].map((c) => <option key={c}>{c}</option>)}
          </select>
        </Field>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Category">
          <select value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })} className={input}>
            {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        </Field>
        <Field label="Brand">
          <select value={form.brand_id} onChange={(e) => setForm({ ...form, brand_id: e.target.value })} className={input}>
            {brands.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
          </select>
        </Field>
      </div>

      <div>
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Sizes (EU)
        </span>
        <div className="flex flex-wrap gap-1.5">
          {Array.from(new Set([...EU_SIZES, ...sizes])).sort((a, b) => a - b).map((sz) => {
            const on = sizes.includes(sz);
            return (
              <button
                key={sz}
                type="button"
                aria-pressed={on}
                onClick={() =>
                  setSizes((cur) => (on ? cur.filter((v) => v !== sz) : [...cur, sz]))
                }
                className={`min-w-[3rem] rounded border px-2.5 py-1.5 text-sm transition ${
                  on
                    ? "border-neutral-900 bg-neutral-900 text-white"
                    : "border-neutral-300 hover:border-neutral-500"
                }`}
              >
                {sz}
              </button>
            );
          })}
        </div>
        <div className="mt-2 flex items-center gap-2">
          <input
            value={otherSize}
            onChange={(e) => setOtherSize(e.target.value)}
            inputMode="decimal"
            placeholder="Other size"
            className="w-32 rounded border border-neutral-300 px-2 py-1 text-sm"
          />
          <button
            type="button"
            onClick={() => {
              const v = Number(otherSize.trim());
              if (Number.isFinite(v) && v > 0 && !sizes.includes(v)) setSizes((c) => [...c, v]);
              setOtherSize("");
            }}
            className="rounded border border-neutral-300 px-3 py-1 text-sm hover:bg-neutral-50"
          >
            Add
          </button>
          <span className="text-xs text-neutral-500">
            {sizes.length} selected — each becomes a buyable variant
          </span>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Colour">
          <select value={color} onChange={(e) => setColor(e.target.value)} className={input}>
            {COLORS.map((c) => <option key={c} value={c}>{c}</option>)}
            <option value="__other">Other…</option>
          </select>
          {color === "__other" && (
            <input
              value={otherColor}
              onChange={(e) => setOtherColor(e.target.value)}
              placeholder="Colour name"
              className={`${input} mt-2`}
            />
          )}
        </Field>
        <Field label="Stock per size">
          <input value={form.stock} inputMode="numeric"
            onChange={(e) => setForm({ ...form, stock: e.target.value })} className={input} />
        </Field>
      </div>

      <div>
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Photos
        </span>
        <button
          type="button"
          disabled={busy}
          onClick={() => photoInput.current?.click()}
          className="rounded border-2 border-dashed border-neutral-400 px-5 py-3 text-sm font-medium text-neutral-800 transition hover:border-neutral-900 hover:bg-neutral-50 disabled:opacity-50"
        >
          {photos.length === 0 ? "+ Add photos" : `+ Add more (${photos.length} selected)`}
        </button>
        <input
          ref={photoInput}
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp"
          className="sr-only"
          onChange={(e) => {
            const picked = Array.from(e.target.files ?? []);
            if (picked.length) setPhotos((cur) => [...cur, ...picked]);
          }}
        />
        {photos.length > 0 && (
          <ul className="mt-3 flex flex-wrap gap-2">
            {photos.map((f, i) => (
              <li key={`${f.name}-${i}`}
                className="flex items-center gap-2 rounded bg-neutral-100 px-2 py-1 text-xs">
                <span className="max-w-[12rem] truncate">{f.name}</span>
                {i === 0 && <span className="text-neutral-500">primary</span>}
                <button type="button" aria-label={`Remove ${f.name}`}
                  onClick={() => setPhotos((cur) => cur.filter((_, j) => j !== i))}
                  className="text-neutral-500 hover:text-neutral-900">×</button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={form.publish}
          onChange={(e) => setForm({ ...form, publish: e.target.checked })} />
        Publish immediately (otherwise saved as a draft, hidden from the storefront)
      </label>

      <button type="button" onClick={submit} disabled={busy || !ready} className={primary}>
        {busy ? (step || "Creating…") : "Create product"}
      </button>

      <Note msg={msg} />
    </section>
  );
}

// ── Categories ────────────────────────────────────────────────────────────────

function Categories() {
  const [cats, setCats] = useState<Category[]>([]);
  const [name, setName] = useState("");
  const [parent, setParent] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);

  const reload = useCallback(async () => setCats(await listCategories()), []);
  useEffect(() => { void reload(); }, [reload]);

  async function add() {
    setBusy(true); setMsg(null);
    try {
      await createCategory(name.trim(), parent || undefined);
      setName(""); setParent("");
      await reload();
      setMsg({ kind: "ok", text: "Category created." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not create the category.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="max-w-2xl space-y-6">
      <div className="space-y-4">
        <Field label="New category">
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Loafers" className={input} />
        </Field>
        <Field label="Parent (optional)">
          <select value={parent} onChange={(e) => setParent(e.target.value)} className={input}>
            <option value="">— top level —</option>
            {cats.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        </Field>
        <button type="button" onClick={add} disabled={busy || !name.trim()} className={primary}>
          {busy ? "Creating…" : "Add category"}
        </button>
        <Note msg={msg} />
      </div>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Existing ({cats.length})
        </h3>
        <ul className="flex flex-wrap gap-2">
          {cats.map((c) => (
            <li key={c.id} className="rounded-full bg-neutral-100 px-3 py-1 text-sm">{c.name}</li>
          ))}
        </ul>
      </div>
    </section>
  );
}

// ── Discounts ─────────────────────────────────────────────────────────────────

function Discounts() {
  const [list, setList] = useState<Promotion[]>([]);
  const [form, setForm] = useState({
    name: "", code: "", discount_type: "percentage" as Promotion["discount_type"],
    discount_value: "", expires_at: "",
  });
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);

  const reload = useCallback(async () => {
    try { setList(await listPromotions()); }
    catch (e) { setMsg({ kind: "error", text: errText(e, "Could not load discounts.") }); }
  }, []);
  useEffect(() => { void reload(); }, [reload]);

  async function create() {
    setBusy(true); setMsg(null);
    const value = Number(form.discount_value);
    if (!Number.isFinite(value) || value < 0) {
      setMsg({ kind: "error", text: "Enter a valid discount value." });
      setBusy(false);
      return;
    }
    try {
      await createPromotion({
        name: form.name.trim(),
        code: form.code.trim() || undefined,
        discount_type: form.discount_type,
        discount_value: value,
        // datetime-local has no zone; treat it as the browser's own.
        expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : undefined,
      });
      setForm({ ...form, name: "", code: "", discount_value: "", expires_at: "" });
      await reload();
      setMsg({ kind: "ok", text: "Discount created." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not create the discount.") });
    } finally { setBusy(false); }
  }

  async function toggle(p: Promotion) {
    try {
      await setPromotionActive(p.id, !p.is_active);
      await reload();
    } catch (e) {
      setMsg({ kind: "error", text: errText(e) });
    }
  }

  return (
    <section className="space-y-8">
      <div className="max-w-2xl space-y-4">
        <Field label="Name">
          <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="e.g. Launch week" className={input} />
        </Field>

        <div className="grid gap-4 sm:grid-cols-3">
          <Field label="Type">
            <select value={form.discount_type} className={input}
              onChange={(e) => setForm({ ...form, discount_type: e.target.value as Promotion["discount_type"] })}>
              <option value="percentage">Percentage</option>
              <option value="fixed_amount">Fixed amount</option>
              <option value="free_shipping">Free shipping</option>
              <option value="buy_one_get_one">Buy one get one</option>
            </select>
          </Field>
          <Field label="Value">
            <input value={form.discount_value} inputMode="decimal"
              onChange={(e) => setForm({ ...form, discount_value: e.target.value })} className={input} />
          </Field>
          <Field label="Code (optional)">
            <input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })}
              placeholder="blank = automatic" className={input} />
          </Field>
        </div>

        <Field label="Ends (optional)">
          <input type="datetime-local" value={form.expires_at}
            onChange={(e) => setForm({ ...form, expires_at: e.target.value })} className={input} />
        </Field>

        <button type="button" onClick={create} disabled={busy || !form.name.trim() || !form.discount_value} className={primary}>
          {busy ? "Creating…" : "Create discount"}
        </button>
        <Note msg={msg} />
      </div>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Discounts ({list.length})
        </h3>
        {list.length === 0 ? (
          <p className="text-sm text-neutral-500">None yet.</p>
        ) : (
          <ul className="space-y-2">
            {list.map((p) => (
              <li key={p.id} className="flex flex-wrap items-center gap-3 rounded border border-neutral-200 px-3 py-2 text-sm">
                <span className="font-medium">{p.name}</span>
                {p.code && <code className="rounded bg-neutral-100 px-1.5 py-0.5 text-xs">{p.code}</code>}
                <span className="text-neutral-600">
                  {p.discount_type === "percentage" ? `${p.discount_value}%` : p.discount_value}
                </span>
                <span className={`rounded-full px-2 py-0.5 text-xs ${
                  p.live ? "bg-green-100 text-green-800" : "bg-neutral-100 text-neutral-600"
                }`}>
                  {p.live ? "live" : p.is_active ? "scheduled / ended" : "off"}
                </span>
                <span className="text-xs text-neutral-500">used {p.uses_count}×</span>
                <button type="button" onClick={() => toggle(p)}
                  className="ml-auto rounded border border-neutral-300 px-2 py-1 text-xs hover:bg-neutral-50">
                  {p.is_active ? "Turn off" : "Turn on"}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

// ── Announcements ─────────────────────────────────────────────────────────────

function Announce() {
  const [form, setForm] = useState({ title: "", body: "", action_url: "" });
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);

  async function send() {
    setBusy(true); setMsg(null);
    try {
      const res = await broadcastNotification({
        title: form.title.trim(),
        body: form.body.trim(),
        action_url: form.action_url.trim() || undefined,
      });
      setForm({ title: "", body: "", action_url: "" });
      setMsg({ kind: "ok", text: `Sent to ${res.recipients} customer${res.recipients === 1 ? "" : "s"}.` });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not send the announcement.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="max-w-2xl space-y-4">
      <p className="text-sm text-neutral-500">
        Appears in the notifications list of every active customer. This does not send email.
      </p>

      <Field label="Title">
        <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} className={input} />
      </Field>
      <Field label="Message">
        <textarea rows={4} value={form.body}
          onChange={(e) => setForm({ ...form, body: e.target.value })} className={input} />
      </Field>
      <Field label="Link (optional)">
        <input value={form.action_url} placeholder="https://www.happyfeetcm.com/sale"
          onChange={(e) => setForm({ ...form, action_url: e.target.value })} className={input} />
      </Field>

      <button type="button" onClick={send} disabled={busy || !form.title.trim() || !form.body.trim()} className={primary}>
        {busy ? "Sending…" : "Send announcement"}
      </button>

      <Note msg={msg} />
    </section>
  );
}
