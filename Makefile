APP := forge
MODULE_DIR := forge
MAIN_PKG := ./cmd/forge
BIN_DIR := bin
DIST_DIR := dist
GO_CACHE_DIR ?= $(CURDIR)/.cache/go-build

VERSION ?= $(shell sed -n 's/^var Version = "\(.*\)"/\1/p' $(MODULE_DIR)/internal/cli/version.go)
GOFLAGS ?=
GOENV := GOCACHE=$(GO_CACHE_DIR)
LDFLAGS := -s -w -X github.com/orot/forge/internal/cli.Version=$(VERSION)
BUILD_OS := $(word 2,$(MAKECMDGOALS))
BUILD_ARCH := $(word 3,$(MAKECMDGOALS))
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

.PHONY: help all build core run install test test-cover fmt fmt-check vet tidy check clean dist
.PHONY: linux mac macos darwin windows amd64 arm64
.PHONY: app gui-dist gui-stage-core gui gui-install gui-start gui-dev gui-clean

help:
	@printf "Forge build targets\n"
	@printf "\n"
	@printf "  make build       Build ./$(BIN_DIR)/$(APP)\n"
	@printf "  make core OS=linux ARCH=amd64\n"
	@printf "                   Build one OS/arch core binary into ./$(DIST_DIR)/core\n"
	@printf "  make build linux Build Linux release binaries\n"
	@printf "  make build mac   Build macOS release binaries\n"
	@printf "  make build windows amd64\n"
	@printf "                   Build a specific OS/architecture release binary\n"
	@printf "  make run ARGS=   Run the CLI with optional args\n"
	@printf "  make install     Install the CLI with go install\n"
	@printf "  make test        Run Go tests\n"
	@printf "  make test-cover  Run tests and write coverage.out\n"
	@printf "  make fmt         Format Go packages\n"
	@printf "  make fmt-check   Verify Go formatting\n"
	@printf "  make vet         Run go vet\n"
	@printf "  make tidy        Tidy go.mod/go.sum\n"
	@printf "  make check       Run fmt-check, vet, and test\n"
	@printf "  make dist        Build release binaries into ./$(DIST_DIR)\n"
	@printf "  make gui-install Install Electron GUI npm deps\n"
	@printf "  make gui-stage-core OS=linux ARCH=amd64\n"
	@printf "                   Build core binary into ./$(GUI_DIR)/resources/bin for GUI packaging\n"
	@printf "  make gui-dist OS=linux ARCH=amd64\n"
	@printf "                   Package Electron GUI using staged core binary\n"
	@printf "  make app OS=linux ARCH=amd64\n"
	@printf "                   Build core, stage it, then package complete Electron app\n"
	@printf "  make gui         Build CLI then launch the Electron GUI\n"
	@printf "  make gui-start   Launch the Electron GUI (assumes CLI is built)\n"
	@printf "  make clean       Remove generated build artifacts\n"

all: build

$(BIN_DIR) $(GO_CACHE_DIR):
	mkdir -p $@

build: $(BIN_DIR) $(GO_CACHE_DIR)
	@if [ -z "$(BUILD_OS)" ]; then \
		$(GOENV) go -C $(MODULE_DIR) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN_DIR)/$(APP) $(MAIN_PKG); \
	else \
		goos="$(BUILD_OS)"; \
		requested_arch="$(BUILD_ARCH)"; \
		case "$$goos" in \
			mac|macos) goos="darwin" ;; \
			linux|darwin|windows) ;; \
			*) echo "unknown build OS: $$goos"; exit 2 ;; \
		esac; \
		if [ -z "$$requested_arch" ]; then \
			case "$$goos" in \
				linux|darwin) archs="amd64 arm64" ;; \
				windows) archs="amd64" ;; \
			esac; \
		else \
			case "$$requested_arch" in \
				amd64|arm64) archs="$$requested_arch" ;; \
				*) echo "unknown build architecture: $$requested_arch"; exit 2 ;; \
			esac; \
		fi; \
		mkdir -p $(DIST_DIR); \
		for arch in $$archs; do \
			ext=""; \
			if [ "$$goos" = "windows" ]; then ext=".exe"; fi; \
			name="$(APP)-$(VERSION)-$$goos-$$arch$$ext"; \
			echo "building $(DIST_DIR)/$$name"; \
			GOOS=$$goos GOARCH=$$arch CGO_ENABLED=0 $(GOENV) go -C $(MODULE_DIR) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o "../$(DIST_DIR)/$$name" $(MAIN_PKG); \
		done; \
	fi

core: $(GO_CACHE_DIR)
	@target_os="$(OS)"; \
	target_arch="$(ARCH)"; \
	case "$$target_os" in \
		mac|macos) target_os="darwin" ;; \
		win) target_os="windows" ;; \
		linux|darwin|windows) ;; \
		*) echo "unknown OS: $$target_os (use linux|darwin|windows)"; exit 2 ;; \
	esac; \
	case "$$target_arch" in \
		amd64|arm64) ;; \
		*) echo "unknown ARCH: $$target_arch (use amd64|arm64)"; exit 2 ;; \
	esac; \
	ext=""; \
	if [ "$$target_os" = "windows" ]; then ext=".exe"; fi; \
	mkdir -p $(DIST_DIR)/core; \
	name="$(APP)-$(VERSION)-$$target_os-$$target_arch$$ext"; \
	echo "building $(DIST_DIR)/core/$$name"; \
	GOOS=$$target_os GOARCH=$$target_arch CGO_ENABLED=0 $(GOENV) go -C $(MODULE_DIR) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o "../$(DIST_DIR)/core/$$name" $(MAIN_PKG)

run: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) run $(MAIN_PKG) $(ARGS)

install: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) install -trimpath -ldflags "$(LDFLAGS)" $(MAIN_PKG)

test: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) test ./...

test-cover: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) test -coverprofile=../coverage.out ./...

fmt: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) fmt ./...

fmt-check:
	@files=$$(find $(MODULE_DIR) -name '*.go' -not -path '*/vendor/*'); \
	unformatted=$$(gofmt -l $$files); \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) vet ./...

tidy: $(GO_CACHE_DIR)
	$(GOENV) go -C $(MODULE_DIR) mod tidy

check: fmt-check vet test

linux mac macos darwin windows amd64 arm64:
	@:

dist:
	$(MAKE) --no-print-directory build linux
	$(MAKE) --no-print-directory build darwin
	$(MAKE) --no-print-directory build windows

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out .cache

# ---------- Electron GUI (Stage 11) ----------

GUI_DIR := gui
GUI_CORE_DIR := $(GUI_DIR)/resources/bin

gui-install:
	cd $(GUI_DIR) && npm install

gui-stage-core: $(GO_CACHE_DIR)
	@target_os="$(OS)"; \
	target_arch="$(ARCH)"; \
	case "$$target_os" in \
		mac|macos) target_os="darwin" ;; \
		win) target_os="windows" ;; \
		linux|darwin|windows) ;; \
		*) echo "unknown OS: $$target_os (use linux|darwin|windows)"; exit 2 ;; \
	esac; \
	case "$$target_arch" in \
		amd64|arm64) ;; \
		*) echo "unknown ARCH: $$target_arch (use amd64|arm64)"; exit 2 ;; \
	esac; \
	ext=""; \
	if [ "$$target_os" = "windows" ]; then ext=".exe"; fi; \
	mkdir -p $(GUI_CORE_DIR); \
	rm -f $(GUI_CORE_DIR)/$(APP) $(GUI_CORE_DIR)/$(APP).exe; \
	echo "staging core for GUI: $(GUI_CORE_DIR)/$(APP)$$ext ($$target_os/$$target_arch)"; \
	GOOS=$$target_os GOARCH=$$target_arch CGO_ENABLED=0 $(GOENV) go -C $(MODULE_DIR) build $(GOFLAGS) -trimpath -ldflags "$(LDFLAGS)" -o "../$(GUI_CORE_DIR)/$(APP)$$ext" $(MAIN_PKG)

gui-dist:
	@target_os="$(OS)"; \
	target_arch="$(ARCH)"; \
	case "$$target_os" in \
		mac|macos|darwin) electron_platform="darwin" ;; \
		linux) electron_platform="linux" ;; \
		win|windows) electron_platform="win32" ;; \
		*) echo "unknown OS: $$target_os (use linux|darwin|windows)"; exit 2 ;; \
	esac; \
	case "$$target_arch" in \
		amd64) electron_arch="x64" ;; \
		arm64) electron_arch="arm64" ;; \
		*) echo "unknown ARCH: $$target_arch (use amd64|arm64)"; exit 2 ;; \
	esac; \
	if [ ! -d "$(GUI_DIR)/node_modules" ]; then \
		echo "GUI dependencies are missing. Run: make gui-install"; \
		exit 2; \
	fi; \
	echo "packaging GUI for $$electron_platform/$$electron_arch"; \
	cd $(GUI_DIR) && npm run make -- --platform=$$electron_platform --arch=$$electron_arch

app: gui-stage-core gui-dist

gui-start:
	cd $(GUI_DIR) && npm start

gui-dev:
	cd $(GUI_DIR) && npm run dev

gui: build gui-start

gui-clean:
	rm -rf $(GUI_DIR)/node_modules $(GUI_CORE_DIR) $(GUI_DIR)/bin $(DIST_DIR)/gui
