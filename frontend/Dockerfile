# ══════════════════════════════════════════════════
# Next.js 15 storefront — standalone production image
# ══════════════════════════════════════════════════

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# `npm ci` needs devDependencies here: the build runs TypeScript, Tailwind and
# sharp. They are dropped in the runtime stage, which copies only the standalone
# output.
RUN npm ci

# ── Build ─────────────────────────────────────────
FROM node:20-alpine AS builder
WORKDIR /app

# NEXT_PUBLIC_* is inlined into the client bundle at BUILD time, not read at
# runtime — so the public API URL has to be known here. It is baked per
# environment; a staging image and a production image are different builds.
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_STORE_NAME
ARG NEXT_PUBLIC_STORE_WHATSAPP_NUMBER
ARG NEXT_PUBLIC_STORE_WHATSAPP_DISPLAY
ARG NEXT_PUBLIC_STORE_SUPPORT_EMAIL
ARG NEXT_PUBLIC_STORE_ADDRESS
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \
    NEXT_PUBLIC_STORE_NAME=$NEXT_PUBLIC_STORE_NAME \
    NEXT_PUBLIC_STORE_WHATSAPP_NUMBER=$NEXT_PUBLIC_STORE_WHATSAPP_NUMBER \
    NEXT_PUBLIC_STORE_WHATSAPP_DISPLAY=$NEXT_PUBLIC_STORE_WHATSAPP_DISPLAY \
    NEXT_PUBLIC_STORE_SUPPORT_EMAIL=$NEXT_PUBLIC_STORE_SUPPORT_EMAIL \
    NEXT_PUBLIC_STORE_ADDRESS=$NEXT_PUBLIC_STORE_ADDRESS \
    NEXT_TELEMETRY_DISABLED=1

COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

# ── Runtime ───────────────────────────────────────
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001

# Static assets and the public folder are NOT included in the standalone
# bundle — Next expects them alongside it.
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000

# Reports unhealthy before the container starts serving errors, so the deploy
# script can roll back rather than cutting traffic to a broken release.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:3000/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"

CMD ["node", "server.js"]
