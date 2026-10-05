#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════════════════
# Pre-launch check. Run from the project root BEFORE the first deploy:
#
#   bash deploy/preflight.sh
#
# Catches the mistakes that are cheap now and expensive after go-live —
# committed secrets, placeholder values left in, missing configuration.
# Exits non-zero if anything must be fixed.
# ══════════════════════════════════════════════════════════════════════════
set -uo pipefail

pass=0; warn=0; fail=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; pass=$((pass+1)); }
no()   { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=$((fail+1)); }
hmm()  { printf '  \033[33m!\033[0m %s\n' "$1"; warn=$((warn+1)); }
head_() { printf '\n\033[1m%s\033[0m\n' "$1"; }

head_ "Repository"
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	ok "git repository initialised"

	# The one that really matters: never ship a secret to a remote.
	leaked=$(git ls-files | grep -E '(^|/)\.env($|\.)' | grep -v '\.example$' || true)
	if [ -n "$leaked" ]; then
		no "SECRETS ARE TRACKED BY GIT:"
		echo "$leaked" | sed 's/^/        /'
		echo "        Fix:  git rm --cached <file>   (then commit)"
	else
		ok "no .env files tracked by git"
	fi

	if git remote get-url origin >/dev/null 2>&1; then
		ok "remote 'origin' is set — $(git remote get-url origin)"
	else
		no "no 'origin' remote — the deploy workflow triggers on push"
	fi

	branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "?")
	[ "$branch" = "main" ] && ok "on branch main" \
		|| hmm "on branch '$branch' — deploys trigger from main"
else
	no "not a git repository — run: git init -b main"
fi

head_ "Required files"
for f in \
	.github/workflows/ci.yml \
	.github/workflows/deploy.yml \
	deploy/docker-compose.prod.yml \
	deploy/Caddyfile \
	deploy/backup.sh \
	backend/Dockerfile \
	frontend/Dockerfile \
	backend/migrations/seed_products.sql
do
	[ -f "$f" ] && ok "$f" || no "missing: $f"
done

head_ "Frontend build config"
if grep -q 'output: *"standalone"' frontend/next.config.ts 2>/dev/null; then
	ok "next.config.ts sets output: standalone"
else
	no "next.config.ts is missing output:\"standalone\" — the image will be ~1.2 GB"
fi

head_ "Local toolchain"
command -v docker >/dev/null 2>&1 && ok "docker $(docker --version | awk '{print $3}' | tr -d ,)" || no "docker not installed"
command -v go     >/dev/null 2>&1 && ok "go $(go version | awk '{print $3}')"                    || no "go not installed"
command -v node   >/dev/null 2>&1 && ok "node $(node -v)"                                        || no "node not installed"

head_ "Placeholders left in development config"
if [ -f backend/.env.local ]; then
	grep -q '^GOOGLE_CLIENT_ID=.*placeholder' backend/.env.local 2>/dev/null \
		&& hmm "backend/.env.local still has the placeholder GOOGLE_CLIENT_ID" \
		|| ok "no placeholder Google client ID"
	grep -q '^STORE_WHATSAPP_NUMBER=237612345678' backend/.env.local 2>/dev/null \
		&& hmm "STORE_WHATSAPP_NUMBER is still the sample number" \
		|| ok "WhatsApp number has been set"
fi

head_ "Reminders — these live on the server or in GitHub, not here"
cat <<'EOF'
    GitHub secrets    DEPLOY_HOST · DEPLOY_USER · DEPLOY_SSH_KEY · DEPLOY_KNOWN_HOSTS
    GitHub variables  DOMAIN · NEXT_PUBLIC_API_URL · NEXT_PUBLIC_STORE_*
    Server file       /opt/happyfeet/.env.prod  (chmod 600)
    Google Console    production origin added to the OAuth client
    DNS               A records for @ and www, "DNS only" until TLS is issued
    Email relays      SMTP_* (Resend) and SMTP2_* (Brevo) in .env.prod, then
                      prove BOTH from the server before launch:
                        docker compose -f docker-compose.prod.yml \
                          --env-file .env.prod run --rm --no-deps api \
                          -mailtest you@example.com
EOF

printf '\n\033[1m%d passed · %d warnings · %d must fix\033[0m\n' "$pass" "$warn" "$fail"
[ "$fail" -eq 0 ] || { echo "Resolve the ✗ items before deploying."; exit 1; }
echo "Ready to deploy."
