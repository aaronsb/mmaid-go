.PHONY: help all build test test-short test-visual clean install lint vet dist release check package version

BINARY := mmaid
BUILD_DIR := .
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

# --- arch-repo's packaging contract ---
#
# https://github.com/aaronsb/arch-repo/blob/main/docs/packaging-contract.md
#
# arch-repo watches this repository, reads ./PKGBUILD from the default branch,
# takes the version and checksum from the newest published release, and pushes
# to the AUR and the [aaronsb] pacman repository. There is deliberately no aur
# target: a second writer to one AUR ref is how a PKGBUILD and its .SRCINFO
# drift apart.

NAME    := $(shell sed -n 's/^pkgname=//p' PKGBUILD)
SRCNAME := $(or $(shell sed -n 's/^_repo=//p' PKGBUILD),$(NAME))

# This project keeps its version in the binary rather than a manifest, because
# `mmaid --version` has to report it.
VERSION := $(shell sed -n 's/^const version = "\(.*\)"/\1/p' cmd/mmaid/main.go)

help: ## List targets
	@grep -hE '^[a-z][a-z-]*:.*##' $(MAKEFILE_LIST) | sed 's/:.*## /\t/' | expand -t20

check: version vet test-short ## Everything CI would run

# PKGBUILD's pkgver is a placeholder arch-repo overwrites, so it is not one of
# the values compared here. What has to agree is the tag about to be cut and the
# version the binary reports. Reporting rather than failing: before a release
# the tag is legitimately absent and after one it is legitimately present.
version: ## Report the version this repository would release
	@test -n "$(VERSION)" || { echo "no const version in cmd/mmaid/main.go" >&2; exit 1; }
	@test -n "$(NAME)"    || { echo "no pkgname in PKGBUILD" >&2; exit 1; }
	@if git rev-parse -q --verify "refs/tags/v$(VERSION)" >/dev/null; then \
	    echo "$(NAME) $(VERSION) — v$(VERSION) is already tagged"; \
	else \
	    echo "$(NAME) $(VERSION) — not yet tagged; this is what the next release will be"; \
	fi

package: version ## Build ./PKGBUILD in a clean chroot and namcap it
	@command -v extra-x86_64-build >/dev/null || { echo "needs devtools" >&2; exit 1; }
	@command -v updpkgsums >/dev/null        || { echo "needs pacman-contrib" >&2; exit 1; }
	@command -v namcap >/dev/null            || { echo "needs namcap" >&2; exit 1; }
	rm -rf pkgbuild-check && mkdir -p pkgbuild-check
	# The tarball the release would carry, built from HEAD and named exactly
	# what source= resolves to, so makepkg uses it instead of reaching for the
	# published archive — which does not exist until the tag does, and would
	# make a pre-release dry run impossible.
	git archive --format=tar.gz --prefix=$(SRCNAME)-$(VERSION)/ \
	    -o pkgbuild-check/$(NAME)-$(VERSION).tar.gz HEAD
	cp PKGBUILD $(wildcard *.install) pkgbuild-check/
	cd pkgbuild-check && sed -i 's/^pkgver=.*/pkgver=$(VERSION)/' PKGBUILD && updpkgsums
	cd pkgbuild-check && extra-x86_64-build
	# namcap exits 0 whether or not it found errors, so its output decides —
	# the same rule arch-repo's gate uses. Debug packages are excluded because
	# every .build-id entry in one is a symlink into the main package.
	cd pkgbuild-check && namcap PKGBUILD $$(ls ./*.pkg.tar.zst | grep -v -- '-debug-') | tee namcap.txt
	@cd pkgbuild-check && if [ -f ../.namcap-allow ]; then \
	    bad=$$(grep ' E: ' namcap.txt | grep -vE -f ../.namcap-allow || true); \
	  else \
	    bad=$$(grep ' E: ' namcap.txt || true); \
	  fi; \
	  if [ -n "$$bad" ]; then echo "namcap errors:"; printf '%s\n' "$$bad"; exit 1; fi; \
	  echo "namcap: no errors"

# The artifacts a release must carry. For the AUR and [aaronsb] that is nothing
# — arch-repo reads the source tarball GitHub generates. The cross-compiled
# binaries below are for everyone not on Arch.
release: dist ## Cross-compile, and cut the GitHub release the packaging reads
ifndef VERSION
	$(error no version found)
endif
	@test -n "$(VERSION)"
	gh release create "v$(VERSION)" --title "v$(VERSION)" --generate-notes \
		$(DIST_DIR)/$(BINARY)-linux-amd64 \
		$(DIST_DIR)/$(BINARY)-linux-arm64 \
		$(DIST_DIR)/$(BINARY)-darwin-amd64 \
		$(DIST_DIR)/$(BINARY)-darwin-arm64 \
		$(DIST_DIR)/$(BINARY)-windows-amd64.exe
	@echo "arch-repo picks this up on its next run; nothing else to do"
