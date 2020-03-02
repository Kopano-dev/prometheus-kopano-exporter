PACKAGE  = stash.kopano.io/kgol/prometheus-kopano-exporter
PACKAGE_NAME = prometheus-kopano-exporter

# Tools

GO      ?= go
GOFMT   ?= gofmt
GOLINT  ?= golangci-lint

# Cgo

CGO_ENABLED ?= 0

# Go modules

GO111MODULE ?= on

# Variables

export CGO_ENABLED GO111MODULE
unexport GOPATH

ARGS    ?=
PWD     := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2>/dev/null | sed 's/^v//' || \
			cat $(CURDIR)/.version 2> /dev/null || echo 0.0.0-unreleased)
PKGS     = $(or $(PKG),$(shell $(GO) list -mod=readonly ./... | grep -v "^$(PACKAGE)/vendor/"))
TESTPKGS = $(shell $(GO) list -mod=readonly -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS) 2>/dev/null)
CMDS     = $(or $(CMD),$(addprefix cmd/,$(notdir $(shell find "$(PWD)/cmd/" -type d))))
TIMEOUT  = 30

# Build

.PHONY: all
all: fmt | $(CMDS) $(PLUGINS)

plugins: fmt | $(PLUGINS)

.PHONY: $(CMDS)
$(CMDS): vendor ; $(info building $@ ...) @
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build \
		-mod=vendor \
		-trimpath \
		-tags release \
		-buildmode=exe \
		-ldflags '-s -w -buildid=reproducible/$(VERSION) -X $(PACKAGE)/version.Version=$(VERSION) -X $(PACKAGE)/version.BuildDate=$(DATE) -extldflags -static' \
		-o bin/$(notdir $@) ./$@

# Helpers

.PHONY: lint
lint: vendor ; $(info running $(GOLINT) ...)	@
	$(GOLINT) run

.PHONY: lint-checkstyle
lint-checkstyle: vendor ; $(info running $(GOLINT) checkstyle ...)     @
	@mkdir -p test
	$(GOLINT) run --out-format checkstyle --issues-exit-code 0 > test/tests.lint.xml

.PHONY: fmt
fmt: ; $(info running gofmt ...)	@
	@ret=0 && for d in $$($(GO) list -mod=readonly -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	done ; exit $$ret

.PHONY: check
check: ; $(info checking dependencies ...) @
	@$(GO) mod verify && echo OK

# Mod

go.sum: go.mod ; $(info updating dependencies ...)
	@$(GO) mod tidy -v
	@touch $@

.PHONY: vendor
vendor: go.sum ; $(info retrieving dependencies ...)
	@$(GO) mod vendor -v
	@touch $@

# Dist

.PHONY: licenses
licenses: vendor ; $(info building licenses files ...)
	$(CURDIR)/scripts/go-license-ranger.py > $(CURDIR)/3rdparty-LICENSES.md

3rdparty-LICENSES.md: licenses

.PHONY: dist
dist: 3rdparty-LICENSES.md ; $(info building dist tarball ...)
	@rm -rf "dist/${PACKAGE_NAME}-${VERSION}"
	@mkdir -p "dist/${PACKAGE_NAME}-${VERSION}"
	@mkdir -p "dist/${PACKAGE_NAME}-${VERSION}/scripts"
	@cd dist && \
	cp -avf ../LICENSE.txt "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../README.md "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../3rdparty-LICENSES.md "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../bin/* "${PACKAGE_NAME}-${VERSION}" && \
	cp -avf ../scripts/prometheus-kopano-exporter.binscript "${PACKAGE_NAME}-${VERSION}/scripts" && \
	cp -avf ../scripts/prometheus-kopano-exporter.service "${PACKAGE_NAME}-${VERSION}/scripts" && \
	cp -avf ../scripts/prometheus-kopano-exporter.cfg "${PACKAGE_NAME}-${VERSION}/scripts" && \
	tar --owner=0 --group=0 -czvf ${PACKAGE_NAME}-${VERSION}.tar.gz "${PACKAGE_NAME}-${VERSION}" && \
	cd ..

.PHONE: changelog
changelog: ; $(info updating changelog ...)
	$(CHGLOG) --output CHANGELOG.md $(ARGS)

# Rest

.PHONY: clean
clean: ; $(info cleaning ...)	@
	@rm -rf bin
	@rm -rf test/test.*

.PHONY: version
version:
	@echo $(VERSION)
