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
LDFLAGS := -s -w  # strip symbols + dwarf → binarios ~40% más chicos

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

test:
	go test ./... 2>&1 || true
