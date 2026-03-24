.PHONY: build test test-visual clean install lint vet dist release aur aur-push

BINARY := mmaid
BUILD_DIR := .
AUR_REPO_DIR ?= $(HOME)/Projects/aur/mmaid
GITHUB_REPO := aaronsb/mmaid-go

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/mmaid

install:
	go install ./cmd/mmaid

test:
	go test ./... -v

test-short:
	go test ./... -short

test-visual: build
	./test_visual.sh ./$(BINARY)

vet:
	go vet ./...

lint: vet
	@echo "Lint passed (go vet)"

clean:
	rm -f $(BINARY)

all: clean build test

# --- Cross-compilation ---

LDFLAGS := -trimpath -ldflags="-s -w"
DIST_DIR := dist

dist:
	@mkdir -p $(DIST_DIR)
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-amd64 ./cmd/mmaid
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-linux-arm64 ./cmd/mmaid
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-amd64 ./cmd/mmaid
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-darwin-arm64 ./cmd/mmaid
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY)-windows-amd64.exe ./cmd/mmaid
	@echo "Built binaries in $(DIST_DIR)/"

# --- AUR packaging ---

# Detect version from latest git tag
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')

# Update PKGBUILD with latest version and checksum, copy to AUR repo
aur:
	@if [ -z "$(VERSION)" ]; then echo "ERROR: No git tag found. Tag a release first."; exit 1; fi
	@echo "Updating PKGBUILD for v$(VERSION)..."
	@TARBALL_URL="https://github.com/$(GITHUB_REPO)/archive/v$(VERSION).tar.gz"; \
	SHA256=$$(curl -sL "$$TARBALL_URL" | sha256sum | awk '{print $$1}'); \
	sed -i "s/^pkgver=.*/pkgver=$(VERSION)/" PKGBUILD; \
	sed -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD; \
	sed -i "s/^sha256sums=.*/sha256sums=('$$SHA256')/" PKGBUILD; \
	echo "SHA256: $$SHA256"
	@echo "PKGBUILD updated."
	@if [ ! -d "$(AUR_REPO_DIR)" ]; then \
		echo "AUR repo not found at $(AUR_REPO_DIR), cloning..."; \
		mkdir -p "$$(dirname "$(AUR_REPO_DIR)")"; \
		git clone "ssh://aur@aur.archlinux.org/mmaid.git" "$(AUR_REPO_DIR)"; \
	fi
	@cp PKGBUILD "$(AUR_REPO_DIR)/"
	@cd "$(AUR_REPO_DIR)" && makepkg --printsrcinfo > .SRCINFO
	@echo "Copied PKGBUILD and generated .SRCINFO in $(AUR_REPO_DIR)"

# Commit and push AUR repo
aur-push: aur
	@cd "$(AUR_REPO_DIR)" && \
	git add PKGBUILD .SRCINFO && \
	git diff --cached --quiet && echo "No changes to push." && exit 0 || true; \
	cd "$(AUR_REPO_DIR)" && \
	git commit -m "Update to $(VERSION)" && \
	git push && \
	echo "Pushed to AUR. Users can install with: yay -S mmaid"

# Full release: cross-compile + GitHub release with binaries + AUR
release: dist
ifndef VERSION
	$(error VERSION is required. Usage: make release VERSION=x.y.z)
endif
	@echo "Creating GitHub release v$(VERSION)..."
	gh release create "v$(VERSION)" --title "v$(VERSION)" --generate-notes \
		$(DIST_DIR)/$(BINARY)-linux-amd64 \
		$(DIST_DIR)/$(BINARY)-linux-arm64 \
		$(DIST_DIR)/$(BINARY)-darwin-amd64 \
		$(DIST_DIR)/$(BINARY)-darwin-arm64 \
		$(DIST_DIR)/$(BINARY)-windows-amd64.exe
	@$(MAKE) aur-push
