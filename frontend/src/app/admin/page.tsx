"use client";

// Minimal catalogue console: pick a product, edit its fields, manage its
// images. Deliberately plain — this is a staff tool, not storefront surface.
//
// The role check here is ergonomics only. Every endpoint it calls is
// admin-scoped server-side and answers 403 regardless of what this renders.

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { me, type AuthUser } from "@/lib/api/auth";
import { listProductsApi } from "@/lib/api/products";
import {
  updateProduct,
  uploadProductImage,
  type ProductPatch,
} from "@/lib/api/admin";
import type { Product } from "@/lib/types";

type Status = "draft" | "active" | "archived";

export default function AdminPage() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);
  const [products, setProducts] = useState<Product[]>([]);
  const [selected, setSelected] = useState<Product | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const u = await me();
      if (!cancelled) {
        setUser(u);
        setChecking(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const reload = useCallback(async () => {
    const list = await listProductsApi({ limit: 100 });
    setProducts(list);
    setSelected((cur) => (cur ? (list.find((p) => p.id === cur.id) ?? cur) : null));
  }, []);

  useEffect(() => {
    if (user?.role === "admin") void reload();
  }, [user, reload]);

  if (checking) {
    return <Shell><p className="text-sm text-neutral-500">Checking your session…</p></Shell>;
  }

  if (!user) {
    return (
      <Shell>
        <p className="text-sm">
          You need to{" "}
          <a href="/account" className="underline">
            sign in
          </a>{" "}
          to use the catalogue console.
        </p>
      </Shell>
    );
  }

  if (user.role !== "admin") {
    return (
      <Shell>
        <p className="text-sm">
          Signed in as <strong>{user.email ?? user.phone}</strong> with the{" "}
          <code className="rounded bg-neutral-100 px-1">{user.role}</code> role.
          Catalogue editing needs the <code className="rounded bg-neutral-100 px-1">admin</code> role.
        </p>
      </Shell>
    );
  }

  return (
    <Shell>
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
                    selected?.id === p.id
                      ? "bg-neutral-900 text-white"
                      : "hover:bg-neutral-100"
                  }`}
                >
                  <span className="block truncate">{p.name}</span>
                  <span className="block truncate text-xs opacity-60">{p.brand}</span>
                </button>
              </li>
            ))}
            {products.length === 0 && (
              <li className="px-3 py-2 text-sm text-neutral-500">No products found.</li>
            )}
          </ul>
        </nav>

        {selected ? (
          <Editor key={selected.id} product={selected} onSaved={reload} />
        ) : (
          <p className="text-sm text-neutral-500">Select a product to edit it.</p>
        )}
      </div>
    </Shell>
  );
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto max-w-6xl px-4 py-10">
      <h1 className="mb-8 text-2xl font-semibold">Catalogue console</h1>
      {children}
    </main>
  );
}

function Editor({ product, onSaved }: { product: Product; onSaved: () => Promise<void> }) {
  const [name, setName] = useState(product.name);
  const [description, setDescription] = useState(product.description ?? "");
  const [price, setPrice] = useState(String(product.price));
  const [status, setStatus] = useState<Status>("active");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  async function save() {
    setBusy(true);
    setMessage(null);

    // Send only what changed — the endpoint treats absent fields as untouched,
    // so a concurrent edit to a field we did not alter survives.
    const patch: ProductPatch = {};
    if (name !== product.name) patch.name = name;
    if (description !== (product.description ?? "")) patch.description = description;
    const parsed = Number(price);
    if (Number.isFinite(parsed) && parsed !== product.price) patch.base_price = parsed;
    patch.status = status;

    try {
      await updateProduct(product.id, patch);
      await onSaved();
      setMessage({ kind: "ok", text: "Saved." });
    } catch (err) {
      setMessage({ kind: "error", text: err instanceof Error ? err.message : "Save failed." });
    } finally {
      setBusy(false);
    }
  }

  async function upload(file: File) {
    setBusy(true);
    setMessage(null);
    try {
      await uploadProductImage(product.id, file, {
        // First image on a product becomes the one shown in listings.
        isPrimary: product.images.length === 0,
        sortOrder: product.images.length,
      });
      await onSaved();
      setMessage({ kind: "ok", text: "Image uploaded." });
      if (fileInput.current) fileInput.current.value = "";
    } catch (err) {
      setMessage({ kind: "error", text: err instanceof Error ? err.message : "Upload failed." });
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="space-y-6">
      <div className="space-y-4">
        <Field label="Name">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full rounded border border-neutral-300 px-3 py-2 text-sm"
          />
        </Field>

        <Field label="Description">
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={4}
            className="w-full rounded border border-neutral-300 px-3 py-2 text-sm"
          />
        </Field>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={`Price (${product.currency})`}>
            <input
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              inputMode="decimal"
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm"
            />
          </Field>

          <Field label="Status">
            <select
              value={status}
              onChange={(e) => setStatus(e.target.value as Status)}
              className="w-full rounded border border-neutral-300 px-3 py-2 text-sm"
            >
              <option value="draft">Draft</option>
              <option value="active">Active</option>
              <option value="archived">Archived</option>
            </select>
          </Field>
        </div>

        <button
          type="button"
          onClick={save}
          disabled={busy}
          className="rounded bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50"
        >
          {busy ? "Working…" : "Save changes"}
        </button>
      </div>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Images ({product.images.length})
        </h3>

        <div className="mb-4 flex flex-wrap gap-3">
          {product.images.map((src) => (
            <Image
              key={src}
              src={src}
              alt=""
              width={96}
              height={96}
              className="h-24 w-24 rounded border border-neutral-200 object-cover"
            />
          ))}
          {product.images.length === 0 && (
            <p className="text-sm text-neutral-500">No images yet.</p>
          )}
        </div>

        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          disabled={busy}
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) void upload(file);
          }}
          className="text-sm"
        />
        <p className="mt-1 text-xs text-neutral-500">
          JPEG, PNG or WebP, up to 10&nbsp;MB. Uploads go straight to Cloudinary.
        </p>
      </div>

      {message && (
        <p
          role="status"
          className={`rounded px-3 py-2 text-sm ${
            message.kind === "ok"
              ? "bg-green-50 text-green-800"
              : "bg-red-50 text-red-800"
          }`}
        >
          {message.text}
        </p>
      )}
    </section>
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
