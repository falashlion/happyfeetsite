# ══════════════════════════════════════════════════
# Stage 1 — Build
#
# TARGETOS/TARGETARCH are supplied by BuildKit. Cross-compiling from them (rather
# than hardcoding amd64) is what lets one `docker buildx` invocation produce both
# the amd64 and arm64 images — the ARM one is what runs on Oracle Ampere.
# ══════════════════════════════════════════════════
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Dependencies first: this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /bin/happyfeet-api \
      ./cmd/api

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" \
      -o /bin/happyfeet-migrate \
      ./cmd/migrate

# ══════════════════════════════════════════════════
# Stage 2 — Run
# ══════════════════════════════════════════════════
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/happyfeet-api /happyfeet-api
COPY --from=builder /bin/happyfeet-migrate /happyfeet-migrate

# Swagger UI is served from ./docs at runtime.
COPY --from=builder /src/docs /docs
# The migrate binary reads .sql files from ./migrations at runtime — without
# these the deploy-time migration step fails with "no such file or directory".
COPY --from=builder /src/migrations /migrations

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/happyfeet-api"]
