# Deploying Happy Feet to Oracle Cloud

From a fresh Always Free instance to a live storefront. Roughly 90 minutes,
most of it waiting for DNS.

Everything below happens once. After it, shipping is `git push origin main` —
GitHub Actions runs the tests, builds both images, migrates the database and
rolls the containers, and rolls back on its own if the new release will not
come up healthy.

**Legend** — 💻 runs on your Mac · 🖥 runs on the server · 🌐 a web console

---

## Phase 0 — Before you start

Have these to hand:

| | |
|---|---|
| Server public IP | Oracle console → Compute → Instances |
| SSH private key | the `.key` file downloaded when you created the instance |
| Login user | `ubuntu` on Ubuntu images, `opc` on Oracle Linux |
| Domain | registrar login, to edit DNS |
| Resend account | you have this |
| Brevo account | you have this |

The instance should be Ubuntu 22.04 or 24.04 on an Ampere A1 shape. If you
picked Oracle Linux, use `opc` as the username — the bootstrap script is
apt-based, so tell me and I will adapt it.

---

## Phase 1 🌐 Open ports 80 and 443 in Oracle

**Do this first.** A VM that is running perfectly but unreachable is the single
most common Oracle Cloud failure, and it looks identical to a broken deploy.

1. Oracle Cloud console → **Networking → Virtual Cloud Networks**
2. Click your VCN → **Security Lists** → **Default Security List**
3. **Add Ingress Rules**, and add these three:

| Stateless | Source CIDR | IP Protocol | Destination Port |
|---|---|---|---|
| No | `0.0.0.0/0` | TCP | 80 |
| No | `0.0.0.0/0` | TCP | 443 |
| No | `0.0.0.0/0` | UDP | 443 |

The UDP rule is HTTP/3. Skip it and the site still works, just without QUIC.

---

## Phase 2 💻 Connect to the server

```bash
# The key must not be readable by anyone else or ssh refuses to use it.
chmod 600 ~/Downloads/ssh-key-*.key
ssh -i ~/Downloads/ssh-key-*.key ubuntu@<SERVER-IP>
```

Make it one word to type from now on — 💻 in `~/.ssh/config`:

```
Host happyfeet
    HostName <SERVER-IP>
    User ubuntu
    IdentityFile ~/.ssh/happyfeet-oracle.key
    ServerAliveInterval 60
```

Move the key there first (`mv ~/Downloads/ssh-key-*.key ~/.ssh/happyfeet-oracle.key`),
then `ssh happyfeet` is enough.

---

## Phase 3 💻🖥 Prepare the server

One script: Docker, firewall, swap, SSH hardening, unattended security updates.
It is idempotent — safe to run twice.

```bash
# 💻 from the project root
scp deploy/bootstrap-server.sh happyfeet:~
ssh happyfeet 'bash bootstrap-server.sh'
```

Then **log out and back in** — Docker group membership does not apply to the
session that created it:

```bash
ssh happyfeet 'docker ps'     # must work without sudo before you continue
```

Keep the `ssh-keyscan` output the script prints at the end. It goes into the
`DEPLOY_KNOWN_HOSTS` secret in Phase 7.

---

## Phase 4 🌐 Point DNS at the server

At your registrar, two A records:

| Type | Name | Value | Proxy |
|---|---|---|---|
| A | `@` | `<SERVER-IP>` | DNS only |
| A | `www` | `<SERVER-IP>` | DNS only |

If you use Cloudflare, the orange cloud must be **off** until TLS is issued —
Caddy proves domain ownership over port 80, and a proxied record intercepts
that challenge. Turn it on afterwards if you want.

Check propagation before moving on:

```bash
dig +short www.yourdomain.cm     # must return your server IP
```

---

## Phase 5 🌐 Email — Resend and Brevo

Both are configured at once. The API round-robins between them, so the daily
ceiling is the sum of the two free tiers rather than the larger of them, and a
provider having a bad day is not the store having a bad day.

### 5a. Resend

1. [resend.com](https://resend.com) → **Domains** → **Add Domain** → your domain
2. Resend shows DNS records (MX, SPF `TXT`, DKIM `TXT`). Add every one at your
   registrar exactly as shown, then press **Verify**. This takes a few minutes.
3. **API Keys** → **Create API Key** → permission *Sending access*. Copy the
   `re_...` value — it is shown once.

SMTP credentials: host `smtp.resend.com`, port `587`, username the literal word
`resend`, password the API key.

> Without a verified domain, Resend only lets you send *to your own account
> address* from `onboarding@resend.dev`. Verify the domain — the store emails
> strangers.

### 5b. Brevo

1. [brevo.com](https://brevo.com) → **Senders, Domains & Dedicated IPs** →
   authenticate the same domain (or at minimum verify `orders@yourdomain.cm` as
   a single sender)
2. **SMTP & API** → **SMTP** tab → **Generate a new SMTP key**

Brevo gives you a login that looks like `9a1b2c001@smtp-brevo.com` and a key
that starts `xsmtpsib-`. The key, **not** your account password.

### 5c. What goes where

| | Resend | Brevo |
|---|---|---|
| Env prefix | `SMTP_` | `SMTP2_` |
| Host | `smtp.resend.com` | `smtp-relay.brevo.com` |
| Port | 587 | 587 |
| Username | `resend` | `9a1b2c001@smtp-brevo.com` |
| Password | `re_...` | `xsmtpsib-...` |
| Free tier | 3,000/month, 100/day | 300/day |

Confirm the current limits on their pricing pages and put them in
`SMTP_DAILY_LIMIT` / `SMTP_MONTHLY_LIMIT` / `SMTP2_DAILY_LIMIT`. They are
advisory: they decide which relay is tried *first*, never whether a message is
sent. A relay that refuses hands the message to the next one.

---

## Phase 6 🖥 Fill in the production secrets

This file holds every secret and lives **only** on the server. It is not in git
and must exist before the first deploy.

```bash
# 💻
scp deploy/.env.prod.example happyfeet:/opt/happyfeet/.env.prod
ssh happyfeet 'chmod 600 /opt/happyfeet/.env.prod'

# 💻 generate the three secrets — hex, not base64: "/" and "+" break the
#    database URL the password is substituted into
openssl rand -hex 32   # → POSTGRES_PASSWORD
openssl rand -hex 48   # → JWT_ACCESS_SECRET
openssl rand -hex 48   # → JWT_REFRESH_SECRET   (a different one)

# 🖥
ssh happyfeet
nano /opt/happyfeet/.env.prod
```

Every field is commented in the file. The ones that must not be left blank:

```bash
DOMAIN=yourdomain.cm                  # apex, no www, no https://
ACME_EMAIL=you@yourdomain.cm          # Let's Encrypt expiry notices
FRONTEND_URL=https://www.yourdomain.cm

POSTGRES_PASSWORD=<openssl rand -hex 32>
JWT_ACCESS_SECRET=<openssl rand -hex 48>
JWT_REFRESH_SECRET=<a different one>

STORE_OWNER_EMAIL=falashcorp@gmail.com      # every new-order alert lands here
STORE_WHATSAPP_NUMBER=237XXXXXXXXX          # digits only — wa.me rejects "+"
STORE_WHATSAPP_DISPLAY=+237 X XX XX XX XX

MAIL_STRATEGY=rotate
SMTP_HOST=smtp.resend.com
SMTP_USERNAME=resend
SMTP_PASSWORD=re_xxxxxxxx
SMTP_FROM=orders@yourdomain.cm              # must be on the verified domain
SMTP2_HOST=smtp-relay.brevo.com
SMTP2_USERNAME=9a1b2c001@smtp-brevo.com
SMTP2_PASSWORD=xsmtpsib-xxxxxxxx

GOOGLE_CLIENT_ID=<same client ID as development>
```

`API_IMAGE` and `WEB_IMAGE` stay empty — the deploy workflow writes them and
rewrites nothing else in this file.

---

## Phase 7 💻🌐 Give GitHub Actions a way in

### 7a. A deploy key, used only by CI

```bash
# 💻 — no passphrase: a CI runner cannot type one
ssh-keygen -t ed25519 -f ~/.ssh/happyfeet-deploy -C "github-actions" -N ""

# install the public half on the server
ssh-copy-id -i ~/.ssh/happyfeet-deploy.pub happyfeet

# prove it works before handing it to CI
ssh -i ~/.ssh/happyfeet-deploy ubuntu@<SERVER-IP> 'echo ok && docker ps -q'
```

### 7b. Repository secrets

GitHub → your repo → **Settings → Secrets and variables → Actions → Secrets →
New repository secret**:

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | `<SERVER-IP>` |
| `DEPLOY_USER` | `ubuntu` |
| `DEPLOY_SSH_KEY` | `cat ~/.ssh/happyfeet-deploy` — the whole file, `BEGIN`/`END` lines included |
| `DEPLOY_KNOWN_HOSTS` | `ssh-keyscan -H <SERVER-IP>` — all lines |

### 7c. Repository variables

Same page, the **Variables** tab. These are compiled into the browser bundle at
build time, so they must be right before the build, not after.

| Variable | Value |
|---|---|
| `DOMAIN` | `yourdomain.cm` |
| `NEXT_PUBLIC_API_URL` | `https://www.yourdomain.cm/api/v1` |
| `NEXT_PUBLIC_STORE_NAME` | `Happy Feet` |
| `NEXT_PUBLIC_STORE_WHATSAPP_NUMBER` | `237XXXXXXXXX` |
| `NEXT_PUBLIC_STORE_WHATSAPP_DISPLAY` | `+237 X XX XX XX XX` |
| `NEXT_PUBLIC_STORE_SUPPORT_EMAIL` | `care@yourdomain.cm` |
| `NEXT_PUBLIC_STORE_ADDRESS` | `Bonapriso, Douala, Cameroon` |

Set these at the **repository** level, not inside an Environment: the job that
builds the storefront image does not run in an environment and would not see
environment-scoped values. The WhatsApp number here and the one in `.env.prod`
must match — the number a customer taps on the site and the one printed in
their confirmation email should be the same number.

---

## Phase 8 🌐 Add the production origin to Google

Google Cloud Console → **APIs & Services → Credentials** → your OAuth client →
**Authorised JavaScript origins** → add:

```
https://www.yourdomain.cm
```

Leave *Authorised redirect URIs* empty; this flow does not use them. If the
consent screen is still in **Testing**, either publish it or add every Google
account that should be able to sign in under **Test users**.

---

## Phase 9 💻 Deploy

```bash
bash deploy/preflight.sh     # catches committed secrets and missing config
git push origin main
```

Watch it on the **Actions** tab. The run is: CI (Go tests with a real Postgres,
typecheck, both image builds) → publish images to GHCR → migrate → roll
containers → smoke-test the public URL.

**The first run takes 10–15 minutes** — multi-arch images with a cold cache.
Later deploys are 3–5.

Certificates are issued on Caddy's first start and renew themselves. If TLS
fails, it is almost always DNS not yet resolving or a Cloudflare proxy left on.

---

## Phase 10 🖥 Go-live checks

```bash
ssh happyfeet
cd /opt/happyfeet

# Everything up?
docker compose -f docker-compose.prod.yml --env-file .env.prod ps

# Load the catalogue (once — the site is empty without it)
docker exec -i happyfeet-postgres-1 psql -U happyfeet -d happyfeet < seed_products.sql

# Prove BOTH relays independently. A normal send only proves that *some*
# relay works, which hides a broken second provider completely.
docker compose -f docker-compose.prod.yml --env-file .env.prod \
  run --rm --no-deps api -mailtest falashcorp@gmail.com
```

Two emails should arrive, subject-lined `… relay test — resend` and
`… relay test — brevo`. Check spam as well as the inbox. A failure names the
relay and the SMTP error.

Then, in a browser:

1. `https://yourdomain.cm` redirects to `https://www.yourdomain.cm`
2. The padlock is green and the catalogue renders
3. Register an account, add a pair to the bag, check out as cash on delivery
4. Both order emails arrive — the merchant alert at `STORE_OWNER_EMAIL` and the
   customer receipt at the account address
5. The WhatsApp buttons in both open a chat with the right number
6. "Continue with Google" signs in

Make yourself an admin — register through the site first, then:

```bash
docker exec -i happyfeet-postgres-1 psql -U happyfeet -d happyfeet -c \
  "UPDATE users SET role='super_admin', status='active', email_verified_at=NOW()
   WHERE email='falashcorp@gmail.com';"
```

---

## Day two

```bash
# Logs
docker compose -f docker-compose.prod.yml --env-file .env.prod logs -f api
docker compose -f docker-compose.prod.yml --env-file .env.prod logs --tail 100 caddy

# Which relay sent what — the API logs "email sent" with a "via" field
docker compose -f docker-compose.prod.yml --env-file .env.prod logs api | grep '"via"'

# Backups: nightly, 14 days, verified gzip
docker exec happyfeet-backup-1 ls -lh /backups

# Restore one
gunzip -c /backups/happyfeet-20260920-030000.sql.gz | \
  docker exec -i happyfeet-postgres-1 psql -U happyfeet -d happyfeet
```

A backup that lives only on the machine it is backing up survives a bad
migration but not a lost VM. Set `S3_BACKUP_BUCKET` in `.env.prod` to push a
copy to Cloudflare R2 (10 GB free, no egress charge).

**Rollback** — every image is tagged with its commit SHA, so the previous
release is still on the host:

```bash
cd /opt/happyfeet
sed -i 's|^API_IMAGE=.*|API_IMAGE=ghcr.io/falashlion/happyfeetsite/api:<old-sha>|' .env.prod
sed -i 's|^WEB_IMAGE=.*|WEB_IMAGE=ghcr.io/falashlion/happyfeetsite/web:<old-sha>|' .env.prod
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
```

Migrations are not reversed by a rollback. Keep them additive — add columns,
never drop them in the same release that stops using them.

**Changing a secret** (an SMTP key, the Google client ID) needs no deploy:

```bash
nano /opt/happyfeet/.env.prod
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --force-recreate api
```

The exception is anything named `NEXT_PUBLIC_*` — those are compiled into the
browser bundle, so change the GitHub *variable* and push to rebuild.

---

## When something is wrong

| Symptom | Where to look |
|---|---|
| Site unreachable, containers healthy | Oracle ingress rules (Phase 1) |
| TLS never issues | `dig +short www.yourdomain.cm`; Cloudflare proxy off; `logs caddy` |
| Deploy fails at "waiting for health" | `logs api` — usually a missing value in `.env.prod` |
| API will not start, database error | `POSTGRES_PASSWORD` contains `/` or `+`; regenerate with `-hex` |
| No email, no error | `SMTP_FROM` is not on the domain the provider verified |
| Google sign-in button missing | `GOOGLE_CLIENT_ID` unset in `.env.prod` — the API answers 503 by design |
| Catalogue empty | the seed in Phase 10 was not run |
