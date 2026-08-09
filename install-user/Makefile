# Makefile — cross-compile el binario domain-install.
#
# Output: dist/domain-install-<os>-<arch>(.exe)
#
# Uso:
#   make             # build linux/amd64 nativo
#   make dist        # cross-compile a los 5 targets
#   make clean
#   make install     # build linux/amd64 + lo copia a /usr/local/bin (sudo)
#
# El binario es estático (sin CGO). Stdlib only, sin deps externas.

BIN := domain-install
DIST := dist

# VERSION sale del tag si hay uno; si no, el binario se declara build de desarrollo. Un
# default con forma de semver haría que el hook SessionStart lo compare contra el server y
# le avise "estás viejo" a quien está compilando el repo (issue-57.1, REQ-1).
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X main.Version=$(VERSION) \
  -X main.Commit=$(COMMIT) \
  -X main.BuildTime=$(BUILD_TIME)

.PHONY: all dist clean install fmt vet test

all:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

dist: clean
	@mkdir -p $(DIST)
	@for t in \
	  linux/amd64    linux/arm64 \
	  darwin/amd64   darwin/arm64 \
	  windows/amd64; do \
	  os=$${t%/*}; arch=$${t#*/}; \
	  ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	  out=$(DIST)/$(BIN)-$$os-$$arch$$ext; \
	  echo "→ $$out"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    go build -ldflags "$(LDFLAGS)" -o $$out . ; \
	done
	@ls -lh $(DIST)/

clean:
	rm -rf $(DIST) $(BIN)

install: all
	sudo install -m 755 $(BIN) /usr/local/bin/$(BIN)

fmt:
	gofmt -w .

vet:
	go vet ./...

# DOMAINSERV-250: el `|| true` descartaba el exit code, así que `make test` salía 0 con la
# suite en rojo. Un target de tests que nunca falla no es un target de tests.
# El -count=1 tampoco es cosmético acá: version_test.go lee archivos de FUERA del módulo con
# paths relativos (../.github/workflows/*.yml), y Go no los rastrea para invalidar el cache,
# así que la suite puede venir entera del cache sin evaluar nada. Es el mismo falso verde que
# ci-mcp.yml ya documenta y por el que ci-install-user.yml corre con -count=1.
test:
	go test -count=1 ./...
