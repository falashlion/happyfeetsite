"use client";

// Staff console: orders, catalogue, categories, discount events, announcements.
// Deliberately plain — this is an internal tool, not storefront surface.
//
// The role check is ergonomics only. Every endpoint it calls is admin-scoped
// server-side and answers 403 regardless of what this chooses to render.

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { me, type AuthUser } from "@/lib/api/auth";
import {
  listAdminProducts, listCategories, listBrands, createCategory, createBrand, createProduct,
  updateProduct, uploadProductImage, listPromotions, createPromotion,
  setPromotionActive, broadcastNotification,
  listAdminOrders, getAdminOrder, updateOrderStatus,
  getAdminProduct, setVariants, detachImage, setPrimaryImage,
  type AdminProduct, type AdminProductDetail, type Category, type Brand, type Promotion,
  type ProductPatch, type ProductFilters, type AdminOrder, type OrderStatus, type OrderFilters,
} from "@/lib/api/admin";
import { formatPrice } from "@/lib/format";
import { COLOR_NAMES, colorHex } from "@/lib/colors";

type Tab = "orders" | "catalogue" | "new" | "categories" | "discounts" | "announce";
type Msg = { kind: "ok" | "error"; text: string } | null;

const TABS: { id: Tab; label: string }[] = [
  { id: "orders", label: "Orders" },
  { id: "catalogue", label: "Catalogue" },
  { id: "new", label: "New product" },
  { id: "categories", label: "Categories & brands" },
  { id: "discounts", label: "Discounts" },
  { id: "announce", label: "Announce" },
];

function errText(e: unknown, fallback = "Something went wrong.") {
  return e instanceof Error && e.message ? e.message : fallback;
}

export default function AdminPage() {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);
  const [tab, setTab] = useState<Tab>("orders");

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

      {tab === "orders" && <Orders />}
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
      <h1 className="mb-6 text-2xl font-semibold">Staff console</h1>
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

const input = "w-full rounded border border-neutral-300 px-3 py-2 text-sm";
const primary = "rounded bg-neutral-900 px-4 py-2 text-sm text-white disabled:opacity-50";

// ── Orders ────────────────────────────────────────────────────────────────────

const STATUS_FILTERS: { value: string; label: string }[] = [
  { value: "PLACED,PAID,PROCESSING,SHIPPED", label: "Open" },
  { value: "all", label: "All" },
  { value: "PLACED", label: "Placed" },
  { value: "PROCESSING", label: "Processing" },
  { value: "SHIPPED", label: "Shipped" },
  { value: "DELIVERED", label: "Delivered" },
  { value: "CANCELLED", label: "Cancelled" },
];

const STATUS_TONE: Record<string, string> = {
  PLACED: "bg-amber-100 text-amber-900",
  PAID: "bg-amber-100 text-amber-900",
  PROCESSING: "bg-blue-100 text-blue-900",
  SHIPPED: "bg-indigo-100 text-indigo-900",
  DELIVERED: "bg-green-100 text-green-800",
  CANCELLED: "bg-neutral-200 text-neutral-700",
};

const STATUS_VERB: Record<string, string> = {
  PROCESSING: "Mark as processing",
  SHIPPED: "Mark as shipped",
  DELIVERED: "Mark as delivered",
  CANCELLED: "Cancel order",
};

function StatusPill({ status }: { status: string }) {
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs ${STATUS_TONE[status] ?? "bg-neutral-100 text-neutral-700"}`}>
      {status.toLowerCase().replace(/_/g, " ")}
    </span>
  );
}

function when(iso: string | null | undefined) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toLocaleString(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

function Orders() {
  const [orders, setOrders] = useState<AdminOrder[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState<Msg>(null);
  const [filters, setFilters] = useState<OrderFilters>({ status: STATUS_FILTERS[0].value, q: "" });

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const page = await listAdminOrders(filters);
      setOrders(page.orders);
      setCursor(page.cursor);
      // Keep the open order selected across a refresh, but drop it if the new
      // filter no longer contains it.
      setSelectedId((cur) => (cur && page.orders.some((o) => o.id === cur) ? cur : null));
      setMsg(null);
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not load orders.") });
    } finally {
      setLoading(false);
    }
  }, [filters]);

  // Debounced so typing in the search box is not one request per keystroke.
  useEffect(() => {
    const id = setTimeout(() => { void reload(); }, 250);
    return () => clearTimeout(id);
  }, [reload]);

  async function loadMore() {
    if (!cursor) return;
    try {
      const page = await listAdminOrders({ ...filters, cursor });
      setOrders((cur) => [...cur, ...page.orders]);
      setCursor(page.cursor);
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not load more orders.") });
    }
  }

  // Replaces one row in place after a status change, so the queue updates
  // without refetching the whole page and losing the operator's scroll.
  const patchRow = useCallback((updated: AdminOrder) => {
    setOrders((cur) => cur.map((o) => (o.id === updated.id ? { ...o, ...updated } : o)));
  }, []);

  const filterBar = (
    <div className="mb-5 flex flex-wrap items-end gap-3">
      <label className="flex-1 min-w-[12rem]">
        <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Search
        </span>
        <input
          value={filters.q ?? ""}
          onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value }))}
          placeholder="Order number, name, email or phone"
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
          {STATUS_FILTERS.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
        </select>
      </label>
    </div>
  );

  if (loading && orders.length === 0) {
    return <div>{filterBar}<p className="text-sm text-neutral-500">Loading orders…</p></div>;
  }

  if (orders.length === 0) {
    return (
      <div>
        {filterBar}
        <p className="text-sm text-neutral-500">
          {filters.q || filters.status !== "all"
            ? "No orders match those filters."
            : "No orders yet."}
        </p>
        <Note msg={msg} />
      </div>
    );
  }

  return (
    <div>
      {filterBar}
      <div className="grid gap-8 md:grid-cols-[20rem_1fr]">
        <nav aria-label="Orders" className="md:border-r md:border-neutral-200 md:pr-6">
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
            Orders ({orders.length})
          </h2>
          <ul className="max-h-[70vh] space-y-1 overflow-y-auto">
            {orders.map((o) => (
              <li key={o.id}>
                <button
                  type="button"
                  onClick={() => setSelectedId(o.id)}
                  aria-current={selectedId === o.id}
                  className={`w-full rounded px-3 py-2 text-left text-sm transition ${
                    selectedId === o.id ? "bg-neutral-900 text-white" : "hover:bg-neutral-100"
                  }`}
                >
                  <span className="flex items-center justify-between gap-2">
                    <span className="truncate font-medium">{o.customer.name || "—"}</span>
                    <span className="shrink-0 text-xs opacity-70">
                      {formatPrice(o.total_amount, o.currency)}
                    </span>
                  </span>
                  <span className="mt-0.5 flex items-center gap-2 text-xs opacity-60">
                    <span className="truncate">{o.order_number}</span>
                    <span className="ml-auto shrink-0">{when(o.placed_at)}</span>
                  </span>
                  <span className="mt-1 block">
                    {selectedId === o.id
                      ? <span className="text-xs opacity-70">{o.status.toLowerCase().replace(/_/g, " ")}</span>
                      : <StatusPill status={o.status} />}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {cursor && (
            <button
              type="button"
              onClick={loadMore}
              className="mt-3 w-full rounded border border-neutral-300 px-3 py-2 text-xs hover:bg-neutral-50"
            >
              Load older orders
            </button>
          )}
          <Note msg={msg} />
        </nav>

        {selectedId ? (
          <OrderDetail key={selectedId} orderId={selectedId} onChanged={patchRow} />
        ) : (
          <p className="text-sm text-neutral-500">Select an order to see it.</p>
        )}
      </div>
    </div>
  );
}

function OrderDetail({
  orderId,
  onChanged,
}: {
  orderId: string;
  onChanged: (o: AdminOrder) => void;
}) {
  const [order, setOrder] = useState<AdminOrder | null>(null);
  const [target, setTarget] = useState<OrderStatus | "">("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);
  // Held separately from `msg` so the forward link survives the next action's
  // message and stays tappable while staff switch to WhatsApp and back.
  const [forwardUrl, setForwardUrl] = useState("");

  useEffect(() => {
    let cancelled = false;
    setOrder(null);
    setForwardUrl("");
    getAdminOrder(orderId)
      .then((o) => { if (!cancelled) { setOrder(o); setTarget(""); } })
      .catch((e) => { if (!cancelled) setMsg({ kind: "error", text: errText(e, "Could not load the order.") }); });
    return () => { cancelled = true; };
  }, [orderId]);

  if (!order) {
    return <section><p className="text-sm text-neutral-500">Loading order…</p><Note msg={msg} /></section>;
  }

  const addr = order.delivery_address ?? {};
  const addressLines = [
    addr.street,
    [addr.city, addr.state, addr.postal_code].filter(Boolean).join(", "),
    addr.country,
  ].filter((l): l is string => Boolean(l && l.trim()));

  async function apply() {
    if (!order || !target) return;
    setBusy(true); setMsg(null); setForwardUrl("");
    try {
      const res = await updateOrderStatus(order.id, target, reason.trim() || undefined);
      setOrder(res);
      onChanged(res);
      setTarget("");
      setReason("");
      if (res.whatsapp_forward_url) setForwardUrl(res.whatsapp_forward_url);

      if (!res.changed) {
        setMsg({ kind: "ok", text: "Already in that status — nothing changed." });
      } else if (res.status === "DELIVERED") {
        setMsg({
          kind: "ok",
          text: order.customer.email
            ? `Marked delivered. A thank-you is on its way to ${order.customer.email}.`
            : "Marked delivered. No email on file — forward the thank-you on WhatsApp below.",
        });
      } else {
        setMsg({ kind: "ok", text: `Moved to ${res.status.toLowerCase()}.` });
      }
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not update the order.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="space-y-6">
      <header className="flex flex-wrap items-center gap-3">
        <h2 className="text-lg font-semibold">{order.order_number}</h2>
        <StatusPill status={order.status} />
        <span className="ml-auto text-sm text-neutral-500">{when(order.placed_at)}</span>
      </header>

      {/* ── Who and where ── */}
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="rounded border border-neutral-200 p-3 text-sm">
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-neutral-500">Customer</h3>
          <p className="font-medium">{order.customer.name || "—"}</p>
          {order.customer.email && (
            <p><a href={`mailto:${order.customer.email}`} className="underline">{order.customer.email}</a></p>
          )}
          {order.customer.phone && (
            <p><a href={`tel:${order.customer.phone}`} className="underline">{order.customer.phone}</a></p>
          )}
          {!order.customer.email && (
            <p className="mt-1 text-xs text-amber-800">No email on file — use WhatsApp to reach them.</p>
          )}
        </div>
        <div className="rounded border border-neutral-200 p-3 text-sm">
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-neutral-500">Deliver to</h3>
          {addr.recipient_name && <p className="font-medium">{addr.recipient_name}</p>}
          {addressLines.map((l) => <p key={l}>{l}</p>)}
          {addr.recipient_phone && <p className="text-neutral-600">{addr.recipient_phone}</p>}
          <p className="mt-2 text-xs text-neutral-500">
            {order.delivery_method.toLowerCase().replace(/_/g, " ")} ·{" "}
            {order.payment_method.replace(/_/g, " ")}
          </p>
        </div>
      </div>

      {/* ── Lines ── */}
      <div>
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-neutral-500">Items</h3>
        <ul className="divide-y divide-neutral-200 border-y border-neutral-200">
          {order.items.map((it) => (
            <li key={it.id} className="flex items-center gap-3 py-2 text-sm">
              {it.primary_image_url && (
                <Image
                  src={it.primary_image_url}
                  alt=""
                  width={40}
                  height={40}
                  className="h-10 w-10 rounded object-cover"
                  unoptimized
                />
              )}
              <span className="min-w-0 flex-1">
                <span className="block truncate">{it.product_name}</span>
                <span className="block text-xs text-neutral-500">
                  EU {it.size_eu} · {it.color} · ×{it.quantity}
                </span>
              </span>
              <span className="shrink-0">{formatPrice(it.subtotal, order.currency)}</span>
            </li>
          ))}
        </ul>
        <dl className="mt-2 space-y-1 text-sm">
          <div className="flex justify-between text-neutral-600">
            <dt>Subtotal</dt><dd>{formatPrice(order.subtotal, order.currency)}</dd>
          </div>
          {order.discount_amount > 0 && (
            <div className="flex justify-between text-neutral-600">
              <dt>Discount</dt><dd>− {formatPrice(order.discount_amount, order.currency)}</dd>
            </div>
          )}
          <div className="flex justify-between text-neutral-600">
            <dt>Delivery</dt>
            <dd>{order.delivery_fee > 0 ? formatPrice(order.delivery_fee, order.currency) : "Complimentary"}</dd>
          </div>
          <div className="flex justify-between font-medium">
            <dt>Total</dt><dd>{formatPrice(order.total_amount, order.currency)}</dd>
          </div>
        </dl>
      </div>

      {/* ── Move it along ── */}
      <div className="rounded border border-neutral-200 p-3">
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Update status
        </h3>
        {order.next_statuses.length === 0 ? (
          <p className="text-sm text-neutral-500">
            {order.status === "DELIVERED"
              ? `Delivered${order.delivered_at ? ` on ${when(order.delivered_at)}` : ""}. Nothing further to do.`
              : "This order has reached a final state."}
          </p>
        ) : (
          <div className="flex flex-wrap items-end gap-3">
            {/* Only the moves the server will accept — the same map gates both ends. */}
            <label className="min-w-[12rem]">
              <span className="sr-only">New status</span>
              <select
                value={target}
                onChange={(e) => setTarget(e.target.value as OrderStatus | "")}
                className={input}
              >
                <option value="">Choose…</option>
                {order.next_statuses.map((s) => (
                  <option key={s} value={s}>{STATUS_VERB[s] ?? s}</option>
                ))}
              </select>
            </label>
            <label className="flex-1 min-w-[12rem]">
              <span className="sr-only">Note</span>
              <input
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Note for the record (optional)"
                className={input}
              />
            </label>
            <button type="button" onClick={apply} disabled={busy || !target} className={primary}>
              {busy ? "Saving…" : "Apply"}
            </button>
          </div>
        )}

        {target === "CANCELLED" && (
          <p className="mt-2 text-xs text-amber-800">
            Cancelling returns these pairs to stock.
          </p>
        )}
        {target === "DELIVERED" && (
          <p className="mt-2 text-xs text-neutral-500">
            Sends the customer a branded thank-you, and offers it for forwarding on WhatsApp.
          </p>
        )}

        <Note msg={msg} />

        {forwardUrl && (
          <a
            href={forwardUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-3 inline-block rounded bg-[#25D366] px-4 py-2 text-sm font-medium text-neutral-900"
          >
            Forward the thank-you on WhatsApp →
          </a>
        )}
      </div>

      {/* ── Audit trail ── */}
      {order.history && order.history.length > 0 && (
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase tracking-wider text-neutral-500">History</h3>
          <ul className="space-y-1 text-sm">
            {order.history.map((h, i) => (
              <li key={`${h.created_at}-${i}`} className="flex flex-wrap items-baseline gap-2 text-neutral-600">
                <span className="text-neutral-900">
                  {h.from_status ? `${h.from_status.toLowerCase()} → ` : ""}{h.to_status.toLowerCase()}
                </span>
                <span className="text-xs">{when(h.created_at)}</span>
                {h.actor_name && <span className="text-xs">by {h.actor_name}</span>}
                {h.reason && <span className="text-xs italic">“{h.reason}”</span>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

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
        <p className="text-sm">Your catalogue is empty.</p>
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

// ── Shared pickers ────────────────────────────────────────────────────────────

const chip = (on: boolean) =>
  `rounded border px-2.5 py-1.5 text-sm transition ${
    on ? "border-neutral-900 bg-neutral-900 text-white" : "border-neutral-300 hover:border-neutral-500"
  }`;

function SizePicker({ sizes, onChange }: { sizes: number[]; onChange: (s: number[]) => void }) {
  const [other, setOther] = useState("");
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
        Sizes (EU)
      </span>
      <div className="flex flex-wrap gap-1.5">
        {Array.from(new Set([...EU_SIZES, ...sizes])).sort((a, b) => a - b).map((sz) => {
          const on = sizes.includes(sz);
          return (
            <button key={sz} type="button" aria-pressed={on} className={`min-w-[3rem] ${chip(on)}`}
              onClick={() => onChange(on ? sizes.filter((v) => v !== sz) : [...sizes, sz])}>
              {sz}
            </button>
          );
        })}
      </div>
      <div className="mt-2 flex items-center gap-2">
        <input value={other} onChange={(e) => setOther(e.target.value)} inputMode="decimal"
          placeholder="Other size" className="w-32 rounded border border-neutral-300 px-2 py-1 text-sm" />
        <button type="button"
          onClick={() => {
            const v = Number(other.trim());
            if (Number.isFinite(v) && v > 0 && !sizes.includes(v)) onChange([...sizes, v]);
            setOther("");
          }}
          className="rounded border border-neutral-300 px-3 py-1 text-sm hover:bg-neutral-50">
          Add
        </button>
        <span className="text-xs text-neutral-500">{sizes.length} selected</span>
      </div>
    </div>
  );
}

function ColorPicker({ colors, onChange }: { colors: string[]; onChange: (c: string[]) => void }) {
  const [other, setOther] = useState("");
  const has = (c: string) => colors.some((x) => x.toLowerCase() === c.toLowerCase());
  const options = [...COLOR_NAMES, ...colors.filter((c) => !COLOR_NAMES.some((n) => n.toLowerCase() === c.toLowerCase()))];
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
        Colours
      </span>
      <div className="flex flex-wrap gap-1.5">
        {options.map((c) => {
          const on = has(c);
          return (
            <button key={c} type="button" aria-pressed={on} className={`flex items-center gap-1.5 ${chip(on)}`}
              onClick={() => onChange(on ? colors.filter((x) => x.toLowerCase() !== c.toLowerCase()) : [...colors, c])}>
              <span aria-hidden className="inline-block h-3 w-3 rounded-full border border-neutral-300"
                style={{ background: colorHex(c) }} />
              {c}
            </button>
          );
        })}
      </div>
      <div className="mt-2 flex items-center gap-2">
        <input value={other} onChange={(e) => setOther(e.target.value)} placeholder="Other colour"
          className="w-40 rounded border border-neutral-300 px-2 py-1 text-sm" />
        <button type="button"
          onClick={() => {
            const v = other.trim();
            if (v && !has(v)) onChange([...colors, v]);
            setOther("");
          }}
          className="rounded border border-neutral-300 px-3 py-1 text-sm hover:bg-neutral-50">
          Add
        </button>
        <span className="text-xs text-neutral-500">
          {colors.length} selected — every size is stocked in each
        </span>
      </div>
    </div>
  );
}

/**
 * The other categories a product also appears under. The primary one is shown
 * ticked and locked: it is the breadcrumb, so it is always included.
 */
function AlsoInCategories({
  cats, primary, extra, onChange,
}: {
  cats: Category[];
  primary: string;
  extra: string[];
  onChange: (ids: string[]) => void;
}) {
  if (cats.length < 2) return null;
  return (
    <div>
      <span className="mb-1 block text-xs font-semibold uppercase tracking-wider text-neutral-500">
        Also show under
      </span>
      <div className="flex flex-wrap gap-1.5">
        {cats.map((c) => {
          const isPrimary = c.id === primary;
          const on = isPrimary || extra.includes(c.id);
          return (
            <button key={c.id} type="button" aria-pressed={on} disabled={isPrimary}
              title={isPrimary ? "Primary category — always included" : undefined}
              className={`${chip(on)} ${isPrimary ? "cursor-default opacity-70" : ""}`}
              onClick={() => onChange(on ? extra.filter((id) => id !== c.id) : [...extra, c.id])}>
              {c.name}{isPrimary ? " · primary" : ""}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ── Editor ────────────────────────────────────────────────────────────────────

const variantKey = (color: string, size: number) => `${color.toLowerCase()}|${size}`;

function Editor({ product, onSaved }: { product: AdminProduct; onSaved: () => Promise<void> }) {
  const [detail, setDetail] = useState<AdminProductDetail | null>(null);
  const [cats, setCats] = useState<Category[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [form, setForm] = useState({
    name: "", description: "", price: "", currency: "XAF",
    category_id: "", brand_id: "", status: "active" as AdminProductDetail["status"], tags: "",
  });
  const [extraCats, setExtraCats] = useState<string[]>([]);
  const [sizes, setSizes] = useState<number[]>([]);
  const [colors, setColors] = useState<string[]>([]);
  // Stock per colour × size, as typed. Combinations not in here start at 0.
  const [stock, setStock] = useState<Record<string, string>>({});
  const [fillAll, setFillAll] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  // Seeds the form from the server's copy, so what you edit is what is stored
  // — not the trimmed-down list row.
  const load = useCallback((d: AdminProductDetail) => {
    setDetail(d);
    setForm({
      name: d.name,
      description: d.description ?? "",
      price: String(d.base_price),
      currency: d.currency,
      category_id: d.category_id,
      brand_id: d.brand_id,
      status: d.status,
      tags: (d.tags ?? []).join(", "),
    });
    setExtraCats((d.category_ids ?? []).filter((id) => id !== d.category_id));
    const skus = d.skus ?? [];
    setSizes(Array.from(new Set(skus.map((s) => s.size_eu))).sort((a, b) => a - b));
    setColors(skus.reduce<string[]>(
      (acc, s) => (acc.some((c) => c.toLowerCase() === s.color.toLowerCase()) ? acc : [...acc, s.color]), []));
    setStock(Object.fromEntries(skus.map((s) => [variantKey(s.color, s.size_eu), String(s.stock_qty)])));
  }, []);

  useEffect(() => {
    let cancelled = false;
    Promise.all([getAdminProduct(product.id), listCategories(), listBrands()])
      .then(([d, c, b]) => {
        if (cancelled) return;
        setCats(c); setBrands(b); load(d);
      })
      .catch((e) => { if (!cancelled) setMsg({ kind: "error", text: errText(e, "Could not load the product.") }); });
    return () => { cancelled = true; };
  }, [product.id, load]);

  if (!detail) {
    return <section><p className="text-sm text-neutral-500">Loading product…</p><Note msg={msg} /></section>;
  }

  const images = [...(detail.images ?? [])].sort(
    (a, b) => Number(b.is_primary) - Number(a.is_primary) || a.sort_order - b.sort_order);

  const variants = colors.flatMap((c) =>
    sizes.map((sz) => ({ size_eu: sz, color: c, stock_qty: Math.max(0, Math.floor(Number(stock[variantKey(c, sz)]) || 0)) })));

  const variantsChanged = (() => {
    const before = (detail.skus ?? [])
      .map((s) => `${variantKey(s.color, s.size_eu)}=${s.stock_qty}`).sort().join(",");
    const after = variants.map((v) => `${variantKey(v.color, v.size_eu)}=${v.stock_qty}`).sort().join(",");
    return before !== after;
  })();

  async function refresh() {
    load(await getAdminProduct(product.id));
    await onSaved();
  }

  async function save() {
    if (!detail) return;
    const price = Number(form.price);
    if (!Number.isFinite(price) || price < 0) {
      setMsg({ kind: "error", text: "Enter a valid price." });
      return;
    }
    if (variants.length === 0) {
      setMsg({ kind: "error", text: "Keep at least one size and one colour — without both it cannot be bought." });
      return;
    }
    setBusy(true); setMsg(null);

    // Send only what changed: absent fields are left untouched server-side, so
    // a concurrent edit to a field we did not alter survives.
    const patch: ProductPatch = {};
    if (form.name.trim() !== detail.name) patch.name = form.name.trim();
    if (form.description.trim() !== (detail.description ?? "")) patch.description = form.description.trim();
    if (price !== detail.base_price) patch.base_price = price;
    if (form.currency !== detail.currency) patch.currency = form.currency;
    if (form.category_id !== detail.category_id) patch.category_id = form.category_id;
    const extra = extraCats.filter((id) => id !== form.category_id);
    const extraBefore = (detail.category_ids ?? []).filter((id) => id !== detail.category_id);
    if (form.category_id !== detail.category_id ||
        [...extra].sort().join() !== [...extraBefore].sort().join()) {
      patch.category_ids = extra;
    }
    if (form.brand_id !== detail.brand_id) patch.brand_id = form.brand_id;
    if (form.status !== detail.status) patch.status = form.status;
    const tags = form.tags.split(",").map((t) => t.trim()).filter(Boolean);
    if (tags.join("\u0000") !== (detail.tags ?? []).join("\u0000")) patch.tags = tags;

    try {
      if (Object.keys(patch).length > 0) await updateProduct(detail.id, patch);
      if (variantsChanged) await setVariants(detail.id, variants);
      await refresh();
      setMsg({ kind: "ok", text: "Saved." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Save failed.") });
    } finally { setBusy(false); }
  }

  async function upload(files: File[]) {
    if (!detail) return;
    setBusy(true); setMsg(null);
    let failed = 0;
    for (const [i, file] of files.entries()) {
      try {
        await uploadProductImage(detail.id, file, {
          // Only the very first photo of a product with none becomes the cover.
          isPrimary: images.length === 0 && i === 0,
          sortOrder: images.length + i,
        });
      } catch {
        failed += 1;
      }
    }
    try { await refresh(); } catch { /* the message below still applies */ }
    setMsg(failed
      ? { kind: "error", text: `${files.length - failed} uploaded, ${failed} failed.` }
      : { kind: "ok", text: files.length === 1 ? "Image uploaded." : `${files.length} images uploaded.` });
    if (fileInput.current) fileInput.current.value = "";
    setBusy(false);
  }

  async function removeImage(imageId: string) {
    if (!detail) return;
    setBusy(true); setMsg(null);
    try {
      await detachImage(detail.id, imageId);
      await refresh();
      setMsg({ kind: "ok", text: "Image removed." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not remove the image.") });
    } finally { setBusy(false); }
  }

  async function makeCover(imageId: string) {
    if (!detail) return;
    setBusy(true); setMsg(null);
    try {
      await setPrimaryImage(detail.id, imageId);
      await refresh();
      setMsg({ kind: "ok", text: "Cover image updated." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not change the cover.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="space-y-6">
      <Field label="Name">
        <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={input} />
      </Field>

      <Field label="Description">
        <textarea rows={4} value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })} className={input} />
      </Field>

      <div className="grid gap-4 sm:grid-cols-3">
        <Field label="Price">
          <input value={form.price} inputMode="decimal"
            onChange={(e) => setForm({ ...form, price: e.target.value })} className={input} />
        </Field>
        <Field label="Currency">
          <select value={form.currency} onChange={(e) => setForm({ ...form, currency: e.target.value })} className={input}>
            {["XAF", "XOF", "USD", "EUR", "GHS", "NGN", "UGX"].map((c) => <option key={c}>{c}</option>)}
          </select>
        </Field>
        <Field label="Status">
          <select value={form.status}
            onChange={(e) => setForm({ ...form, status: e.target.value as AdminProductDetail["status"] })}
            className={input}>
            <option value="draft">Draft</option>
            <option value="active">Active</option>
            <option value="archived">Archived</option>
          </select>
        </Field>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Primary category">
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

      <AlsoInCategories cats={cats} primary={form.category_id} extra={extraCats} onChange={setExtraCats} />

      <Field label="Tags (comma-separated — the first is shown under the name)">
        <input value={form.tags} onChange={(e) => setForm({ ...form, tags: e.target.value })}
          placeholder="e.g. Suede slip-on, Leather sole" className={input} />
      </Field>

      <div className="space-y-4 border-t border-neutral-200 pt-6">
        <h3 className="text-xs font-semibold uppercase tracking-wider text-neutral-500">Sizes, colours & stock</h3>
        <SizePicker sizes={sizes} onChange={setSizes} />
        <ColorPicker colors={colors} onChange={setColors} />

        {variants.length > 0 && (
          <div>
            <div className="mb-2 flex items-center gap-2 text-xs text-neutral-500">
              <span>Set every variant to</span>
              <input value={fillAll} onChange={(e) => setFillAll(e.target.value)} inputMode="numeric"
                className="w-20 rounded border border-neutral-300 px-2 py-1 text-sm" />
              <button type="button"
                onClick={() => {
                  const n = String(Math.max(0, Math.floor(Number(fillAll) || 0)));
                  setStock((cur) => ({
                    ...cur,
                    ...Object.fromEntries(variants.map((v) => [variantKey(v.color, v.size_eu), n])),
                  }));
                }}
                className="rounded border border-neutral-300 px-3 py-1 text-sm hover:bg-neutral-50">
                Apply
              </button>
            </div>
            <div className="overflow-x-auto">
              <table className="text-sm">
                <thead>
                  <tr>
                    <th className="pr-3 text-left text-xs font-normal text-neutral-500">EU</th>
                    {[...sizes].sort((a, b) => a - b).map((sz) => (
                      <th key={sz} className="px-1 text-xs font-normal text-neutral-500">{sz}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {colors.map((c) => (
                    <tr key={c}>
                      <td className="whitespace-nowrap pr-3 py-1">
                        <span aria-hidden className="mr-1.5 inline-block h-3 w-3 rounded-full border border-neutral-300 align-middle"
                          style={{ background: colorHex(c) }} />
                        {c}
                      </td>
                      {[...sizes].sort((a, b) => a - b).map((sz) => {
                        const k = variantKey(c, sz);
                        return (
                          <td key={sz} className="px-1 py-1">
                            <input aria-label={`Stock for ${c}, EU ${sz}`} value={stock[k] ?? "0"} inputMode="numeric"
                              onChange={(e) => setStock((cur) => ({ ...cur, [k]: e.target.value }))}
                              className="w-14 rounded border border-neutral-300 px-1.5 py-1 text-right text-sm" />
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <p className="mt-1 text-xs text-neutral-500">
              Removing a size or colour hides those variants; past orders that contain them are unaffected.
            </p>
          </div>
        )}
      </div>

      <button type="button" onClick={save} disabled={busy} className={primary}>
        {busy ? "Working…" : "Save changes"}
      </button>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Images ({images.length})
        </h3>

        <ul className="mb-4 flex flex-wrap gap-3">
          {images.map((img) => {
            const src = img.url_medium || img.url_thumbnail || img.url_original;
            return (
              <li key={img.id} className="w-28">
                <div className="relative">
                  <Image src={src} alt="" width={112} height={112} unoptimized
                    className={`h-28 w-28 rounded border object-cover ${img.is_primary ? "border-neutral-900 border-2" : "border-neutral-200"}`} />
                  {img.is_primary && (
                    <span className="absolute left-1 top-1 rounded bg-neutral-900 px-1.5 py-0.5 text-[10px] uppercase tracking-wider text-white">
                      Cover
                    </span>
                  )}
                </div>
                <div className="mt-1 flex justify-between gap-1 text-xs">
                  {!img.is_primary ? (
                    <button type="button" disabled={busy} onClick={() => makeCover(img.id)}
                      className="underline disabled:opacity-50">Make cover</button>
                  ) : <span />}
                  <button type="button" disabled={busy} onClick={() => removeImage(img.id)}
                    className="text-red-700 underline disabled:opacity-50">Remove</button>
                </div>
              </li>
            );
          })}
          {images.length === 0 && <li className="text-sm text-neutral-500">No images yet.</li>}
        </ul>

        {/* A bare file input reads as nothing at all; this is the primary action
            on this panel, so it gets a real button. */}
        <button
          type="button"
          disabled={busy}
          onClick={() => fileInput.current?.click()}
          className="rounded border-2 border-dashed border-neutral-400 px-5 py-3 text-sm font-medium text-neutral-800 transition hover:border-neutral-900 hover:bg-neutral-50 disabled:opacity-50"
        >
          {busy ? "Working…" : images.length === 0 ? "+ Upload images" : "+ Add more images"}
        </button>
        <input
          ref={fileInput}
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp"
          className="sr-only"
          onChange={(e) => {
            const picked = Array.from(e.target.files ?? []);
            if (picked.length) void upload(picked);
          }}
        />
        <p className="mt-2 text-xs text-neutral-500">
          JPEG, PNG or WebP, up to 10&nbsp;MB each. Uploads go straight to Cloudinary.
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
  const [colors, setColors] = useState<string[]>(["Black"]);
  const [extraCats, setExtraCats] = useState<string[]>([]);
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
    if (sizes.length === 0 || colors.length === 0) {
      setMsg({ kind: "error", text: "Pick at least one size and one colour — without both it cannot be bought." });
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
        category_ids: extraCats.filter((id) => id !== form.category_id),
        brand_id: form.brand_id,
        sizes: [...sizes].sort((a, b) => a - b),
        colors,
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
        <Field label="Primary category">
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

      <AlsoInCategories cats={cats} primary={form.category_id} extra={extraCats} onChange={setExtraCats} />

      <SizePicker sizes={sizes} onChange={setSizes} />

      <ColorPicker colors={colors} onChange={setColors} />

      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Stock per size & colour">
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
    <div className="space-y-12">
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
            Existing categories ({cats.length})
          </h3>
          <ul className="flex flex-wrap gap-2">
            {cats.map((c) => (
              <li key={c.id} className="rounded-full bg-neutral-100 px-3 py-1 text-sm">{c.name}</li>
            ))}
          </ul>
        </div>
      </section>

      <Brands />
    </div>
  );
}

function Brands() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<Msg>(null);

  const reload = useCallback(async () => setBrands(await listBrands()), []);
  useEffect(() => { void reload(); }, [reload]);

  async function add() {
    setBusy(true); setMsg(null);
    try {
      await createBrand(name.trim());
      setName("");
      await reload();
      setMsg({ kind: "ok", text: "Brand created. It is now in the brand list on New product." });
    } catch (e) {
      setMsg({ kind: "error", text: errText(e, "Could not create the brand.") });
    } finally { setBusy(false); }
  }

  return (
    <section className="max-w-2xl space-y-6 border-t border-neutral-200 pt-10">
      <div className="space-y-4">
        <Field label="New brand">
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Loro Piana"
            onKeyDown={(e) => { if (e.key === "Enter" && name.trim() && !busy) void add(); }}
            className={input} />
        </Field>
        <button type="button" onClick={add} disabled={busy || !name.trim()} className={primary}>
          {busy ? "Creating…" : "Add brand"}
        </button>
        <Note msg={msg} />
      </div>

      <div className="border-t border-neutral-200 pt-6">
        <h3 className="mb-3 text-xs font-semibold uppercase tracking-wider text-neutral-500">
          Existing brands ({brands.length})
        </h3>
        <ul className="flex flex-wrap gap-2">
          {brands.map((b) => (
            <li key={b.id} className="rounded-full bg-neutral-100 px-3 py-1 text-sm">{b.name}</li>
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
