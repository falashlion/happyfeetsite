# Happy Feet

Luxury footwear storefront for Cameroon — a Next.js 15 storefront over a Go API,
with Postgres, Redis and transactional email.

```
happfeetsite/
├── frontend/        Next.js 15 · React 19 · TypeScript · Tailwind 4 · Zustand
├── backend/         Go 1.22 · chi · pgx · Redis · golang-migrate
├── Makefile         workspace tasks — start here
└── README.md
```

---

## Quick start

```bash
make install     # once: frontend dependencies
make dev         # infra + migrations + API + storefront
```

| | |
|---|---|
| Storefront | http://localhost:3000 |
| API | http://localhost:8080/api/v1 |
| API reference | http://localhost:8080/swagger |
| Captured email | http://localhost:8025 |
| Postgres | `localhost:5433` · `happyfeet` / `happyfeet_dev_secret` |
| Redis | `localhost:6380` |

`make stop` shuts everything down, `make status` reports what is up, and
`make logs-api` / `make logs-web` tail the two services. `make help` lists the
rest.

The ports are deliberately off-default (5433, 6380) so the stack can run beside
other local Postgres and Redis instances.

---

## Configuration

Real environment variables always win. Below them, the API reads `.env` and then
`.env.local` from `backend/`, with `.env.local` overriding `.env`. That split is
what lets the same `.env` serve both Docker (`postgres:5432`) and a host-run
binary (`localhost:5433`) — see `backend/pkg/config/dotenv.go`.

| File | Purpose |
|---|---|
| `backend/.env` | Docker-network defaults |
| `backend/.env.local` | Host overrides + store identity + SMTP |
| `backend/.env.example` | Documented template |
| `frontend/.env.local` | API URL and `NEXT_PUBLIC_STORE_*` contact details |

### Store identity

These drive the order emails and every WhatsApp link. The `STORE_*` values in
`backend/.env.local` and the `NEXT_PUBLIC_STORE_*` values in
`frontend/.env.local` **must match** — the number a customer taps on the site and
the one printed in their confirmation email should be the same number.

```bash
STORE_OWNER_EMAIL=falashcorp@gmail.com   # receives every new-order alert
STORE_WHATSAPP_NUMBER=237612345678       # digits only — wa.me rejects "+" and spaces
STORE_WHATSAPP_DISPLAY=+237 6 12 34 56 78
```

---

## Payment: cash on delivery

Online payment is **switched off in the storefront only**. Every order is placed
as `cash_on_delivery`, and the confirmation page tells the customer what to have
ready at the door.

The API is untouched: `/payments/initiate` still accepts `mtn_momo`,
`orange_money`, `stripe` and `cash_on_delivery`, and all three provider webhooks
remain mounted. Re-enabling is a frontend edit in
`frontend/src/app/checkout/checkout-client.tsx`:

1. restore the `const [payment, setPayment] = useState(...)` hook (commented out
   just above the delivery state), and
2. un-comment the payment `RadioCard` group in step three.

Both blocks are left in place and commented, not deleted.

---

## Order email

Placing an order sends two messages, rendered from the templates in
`backend/internal/notification/templates/`:

- **Merchant alert** → `STORE_OWNER_EMAIL`. Amount to collect, customer and
  delivery address, what to pack, and a one-tap **"Message _customer_ on
  WhatsApp"** button pre-filled with the delivery confirmation. `Reply-To` is set
  to the customer, so replying from the inbox reaches them directly.
- **Customer receipt** → the address on their account. Order number, amount due
  on delivery, their selection, and a **"Chat with us on WhatsApp"** button
  pre-filled with their order number and total.

Both are `multipart/alternative` (HTML + plain text), table-based for email-client
compatibility, and responsive down to 320px.

Delivery is asynchronous and failure-tolerant: a slow or broken relay logs an
error and never fails the order. With `SMTP_HOST` unset, email is simply off and
the API logs a warning at boot.

### Local capture

`make dev` starts [Mailpit](https://mailpit.axllent.org). Every message is
captured at `localhost:1025` and readable at **http://localhost:8025** — nothing
leaves your machine, so you can iterate on the templates against real orders.

### Sending to a real inbox

Point the relay at a real service. Gmail needs an **App Password** (16
characters, 2-Step Verification enabled) — the account password is rejected:

```bash
# backend/.env.local
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587                  # 587 STARTTLS · 465 implicit TLS
SMTP_USERNAME=falashcorp@gmail.com
SMTP_PASSWORD=<16-char app password>
SMTP_FROM=falashcorp@gmail.com
```

Restart the API. For production volume prefer a transactional provider — Gmail
rate-limits and will mark bulk sending as abuse.

### Two relays at once

`SMTP2_*` (and `SMTP3_*`) add more relays. Two free tiers side by side send far
more than either alone, and one provider's outage stops being the store's
outage:

```bash
MAIL_STRATEGY=rotate                  # or: failover

SMTP_HOST=smtp.resend.com             # Resend — 3,000/month free
SMTP_PORT=587
SMTP_USERNAME=resend                  # literally the word "resend"
SMTP_PASSWORD=re_xxxxxxxx             # the API key
SMTP_FROM=orders@yourdomain.cm        # must be on the domain verified in Resend
SMTP_MONTHLY_LIMIT=3000

SMTP2_HOST=smtp-relay.brevo.com       # Brevo — 300/day free
SMTP2_PORT=587
SMTP2_USERNAME=9a1b2c001@smtp-brevo.com
SMTP2_PASSWORD=xsmtpsib-xxxxxxxx      # SMTP key, not the account password
SMTP2_DAILY_LIMIT=300
```

| | |
|---|---|
| `rotate` (default) | round-robin — both allowances actually get used |
| `failover` | stay on the primary; reach for the second only when it fails or is spent |

Either way, a message one relay refuses is retried on the next before it is
reported as failed, so a bad API key or a spent quota costs latency rather than
an order alert. The `*_LIMIT` values are advisory: they are counted in memory
(and forgotten on restart) and only decide which relay is *tried first* — the
relay itself is the authority on whether it will accept the message.

`SMTP2_FROM` / `SMTP2_FROM_NAME` default to the primary's. Set them only when
the second provider has verified a different sending domain — each relay signs
with its own `From`, because a provider damages or rejects a sender it has not
verified.

Verify every relay independently — with failover in play a broken second
provider is otherwise invisible:

```bash
cd backend && go run ./cmd/api -mailtest you@example.com
```

It sends one message through *each* configured relay and reports per-relay
success, naming the provider in the subject line.

---

## Sign in with Google

The account page offers "Continue with Google" alongside email/password. The
flow is ID-token based — no client secret, no redirect, no callback route:

1. Google Identity Services hands the browser a signed ID token.
2. The storefront posts it to `POST /auth/social/google`.
3. The API verifies the RS256 signature against Google's published keys, then
   checks `iss`, `aud` (our client ID) and `exp`, and that the email is
   verified.
4. On first sign-in the account is created; afterwards it is matched on Google's
   immutable `sub`. `201` means created, `200` means an existing account.

Accounts are linked, never duplicated: an existing email/password account signing
in with the same verified Google address attaches to that account.

### Setup

Create an OAuth client — five minutes, once:

1. [Google Cloud Console](https://console.cloud.google.com) → create or pick a project.
2. **APIs & Services → OAuth consent screen** — External, fill in app name and
   support email. While the app is in Testing, add the Google accounts you want
   to sign in with under **Test users**.
3. **APIs & Services → Credentials → Create credentials → OAuth client ID** →
   application type **Web application**.
4. Under **Authorised JavaScript origins** add every origin the storefront is
   served from — `http://localhost:3000` for development. (Leave *Authorised
   redirect URIs* empty; this flow does not use them.)
5. Copy the Client ID into `backend/.env.local`:

```bash
GOOGLE_CLIENT_ID=1234567890-abc123.apps.googleusercontent.com
```

Then `make api`. The storefront discovers the client ID from
`GET /auth/providers`, so it needs no configuration of its own.

Leave `GOOGLE_CLIENT_ID` unset and the feature is simply absent: the button is
hidden and the endpoint answers `503`. It never fails open.

### Testing it

```bash
make infra-up
cd backend && go test ./pkg/oauth/... ./internal/auth/... -run Google -v
```

`pkg/oauth` covers the verifier against a stand-in Google — including the
rejections that matter: a token minted for a *different* application, a forged
signature, `alg:none`, RS256→HS256 algorithm confusion, an expired or
never-expiring token, and an unverified email address.

`internal/auth` runs the real handler against the real Postgres from
`compose.local.yml` and asserts what actually lands in the database: the user
row, the `user_social_auth` link, no password hash, a pre-verified email, and
that signing in twice yields one account rather than two. It skips itself when
the local stack is not running.

---

## Responsiveness

The storefront is verified from **320px to 1920px** with no horizontal overflow
on any route. Notable behaviour:

- Navigation collapses to a drawer below **1024px** — the six category links plus
  the locale toggle and three actions only fit above that.
- Product grid: 1 column ≤400px, 2 columns ≤760px, auto-fill above.
- Checkout drops to one column ≤900px and the order summary stops sticking, so a
  short screen is not consumed by it.
- Touch targets are at least 44px under `(pointer: coarse)`, and hover-only
  affordances (the quick-add overlay) stay visible on touch.
- `prefers-reduced-motion` is honoured globally.

Breakpoints live in `frontend/src/styles/globals.css`. Where a component's Tailwind
utility and a CSS media query must agree — the header, notably — both are noted in
comments; change them together.

---

## Deployment

The whole stack runs on one Docker host behind Caddy, which terminates TLS and
serves the API and the storefront from a single origin. Push to `main` and
GitHub Actions runs the tests, builds both images, migrates, rolls the
containers and rolls back on its own if the release will not come up healthy.

**[deploy/DEPLOY.md](deploy/DEPLOY.md)** is the first-time runbook — Oracle
instance to live storefront, in order, with the failure modes each step is
guarding against. Before the first push:

```bash
bash deploy/preflight.sh
```

---

## Development

```bash
make check       # go vet + go test + tsc --noEmit
make test        # Go tests
make typecheck   # storefront types
make build       # production build of both halves
make psql        # database shell
make seed        # reload the demo catalogue
```

### Architecture notes

- **Auth** is JWT access/refresh; the storefront keeps tokens in `localStorage`
  and attaches them in `frontend/src/lib/api/client.ts`.
- **The bag** is local-first (Zustand, persisted). At checkout it is pushed to the
  server cart and the order is built from that, so pricing and stock are always
  decided server-side.
- **Order creation** and **payment initiation** both require an `Idempotency-Key`
  header; the client generates one per attempt.
- **i18n** is a flat EN/FR dictionary (`frontend/src/lib/i18n/dict.ts`). EN is
  authoritative and missing keys fall back loudly rather than silently.
