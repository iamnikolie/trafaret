# trafaret — build & dev tasks. Run `make help` to list targets.
BIN     := trafaret
PREFIX  ?= $(HOME)/.local/bin
PKGS    := ./...
PKG     := github.com/iamnikolie/trafaret/cmd
# Stamped into the binary (`trafaret version`, manifest `engine:` gate). Unstamped = "dev".
VERSION ?= $(shell git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG).version=$(VERSION)

.DEFAULT_GOAL := build
.PHONY: build install uninstall run test vet fmt tidy check site clean help

## build: compile the trafaret binary into ./$(BIN)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

## install: build, then symlink $(PREFIX)/$(BIN) -> ./$(BIN) (rebuild is enough to update)
install: build
	mkdir -p $(PREFIX)
	ln -sf $(CURDIR)/$(BIN) $(PREFIX)/$(BIN)
	@echo "installed: $(PREFIX)/$(BIN) -> $(CURDIR)/$(BIN)"

## uninstall: remove the installed symlink
uninstall:
	rm -f $(PREFIX)/$(BIN)

## run: build and run (pass args via ARGS=, e.g. make run ARGS="install myproject --dry-run")
run: build
	./$(BIN) $(ARGS)

## test: run the unit test suite (with the race detector, as CI does)
test:
	go test -race $(PKGS)

## vet: run go vet
vet:
	go vet $(PKGS)

## fmt: gofmt all Go sources in place
fmt:
	gofmt -w .

## tidy: go mod tidy
tidy:
	go mod tidy

## check: fmt + vet + test
check: fmt vet test

## site: serve the live operator manual on localhost (renders manifest/templates/projects per refresh)
site: build
	@( sleep 0.7; open "http://127.0.0.1:4477" 2>/dev/null || true ) &
	./$(BIN) serve

## clean: remove the built binary and release output
clean:
	rm -f $(BIN)
	rm -rf dist

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | sort
