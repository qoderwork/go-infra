# go-infra development tasks.
#
# On Windows with MinGW-w64 8.1.0, a bare `go test -race` fails to start with
# 0xC0000139 (STATUS_ENTRYPOINT_NOT_FOUND): the race runtime resolves
# WaitOnAddress/WakeByAddress* from libkernel32.a into kernel32.dll, which
# does not export them on every Windows build. Linking libsynchronization.a
# routes those imports through the api-ms-win-core-synch-l1-2-0 API set,
# which is always present. Linux/macOS need none of this.

GO ?= go

RACE_ENV :=
ifeq ($(OS),Windows_NT)
RACE_ENV := CGO_CFLAGS="-O2 -g" CGO_LDFLAGS="-lsynchronization"
endif

CONCURRENCY_PKGS := ./structured/ ./workerpool/ ./conc/ ./event/ ./queue/ ./lifecycle/ ./hook/

.PHONY: test test-race test-race-repeat coverage api vet build check

test:
	$(GO) test ./... -count=1

test-race:
	$(RACE_ENV) $(GO) test -race ./... -count=1 -timeout 240s

test-race-repeat:
	$(RACE_ENV) $(GO) test -race -count=5 $(CONCURRENCY_PKGS)

coverage:
	bash scripts/check-coverage.sh

api:
	bash scripts/check-api.sh HEAD^

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

check: vet test test-race coverage
