.PHONY: build run test test-race coverage lint swagger migrate-up migrate-down docker clean

# ── Variables ─────────────────────────────────────
APP       = happyfeet-api
CMD       = ./cmd/api/main.go
MIGRATE   = ./cmd/migrate/main.go
GOFLAGS   = -mod=mod
GONOSUMDB = *
GOPROXY   = direct
GO        = GONOSUMDB=$(GONOSUMDB) GOFLAGS="$(GOFLAGS)" GOPROXY=$(GOPROXY) go

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   = -ldflags "-s -w -X main.version=$(VERSION)"

# ── Build ─────────────────────────────────────────
build:
	$(GO) build $(LDFLAGS) -o $(APP) $(CMD)
	$(GO) build $(LDFLAGS) -o happyfeet-migrate $(MIGRATE)
	@echo "✓ Built $(APP) and happyfeet-migrate"

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(APP)-linux-amd64 $(CMD)

# ── Run ───────────────────────────────────────────
run:
	$(GO) run $(CMD)

# ── Test ──────────────────────────────────────────
test:
	$(GO) test ./... -count=1

test-race:
	$(GO) test ./... -count=1 -race

coverage:
	$(GO) test ./... -count=1 -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# ── Database ──────────────────────────────────────
migrate-up:
	$(GO) run $(MIGRATE) -direction up

migrate-down:
	$(GO) run $(MIGRATE) -direction down -steps 1

migrate-version:
	$(GO) run $(MIGRATE) -direction version

# ── Docker ────────────────────────────────────────
docker-up:
	docker compose up -d postgres redis minio
	@echo "Waiting for services..." && sleep 3
	docker compose run --rm migrate
	docker compose up -d api

docker-down:
	docker compose down

docker-build:
	docker build -t happyfeet/api:$(VERSION) .

# ── Lint ──────────────────────────────────────────
lint:
	$(GO) vet ./...
	@which staticcheck > /dev/null && staticcheck ./... || echo "Install: go install honnef.co/go/tools/cmd/staticcheck@latest"

# ── Clean ─────────────────────────────────────────
clean:
	rm -f $(APP) happyfeet-migrate coverage.out coverage.html
	$(GO) clean -cache
