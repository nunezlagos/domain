# Domain multi-stage build (HU-19.3).
# - builder: compila binary del cmd/domain con flags optimizados
# - runtime: distroless con HEALTHCHECK
#
# Tags: ghcr.io/nunezlagos/domain:<version>
# Build local: docker buildx build -t domain:dev --load .

FROM golang:1.25-alpine AS builder

# Build-time args provistos por goreleaser o CI
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src

# go.mod/sum primero para cache de deps
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary, sin CGO, optimizado
ENV CGO_ENABLED=0 GOOS=linux

RUN go build \
    -ldflags="-s -w \
      -X main.Version=${VERSION} \
      -X main.Commit=${COMMIT} \
      -X main.BuildTime=${BUILD_TIME}" \
    -o /out/domain ./cmd/domain && \
    go build \
    -ldflags="-s -w \
      -X main.Version=${VERSION} \
      -X main.Commit=${COMMIT} \
      -X main.BuildTime=${BUILD_TIME}" \
    -o /out/domain-mcp ./cmd/domain-mcp

# Runtime distroless: 22MB total, no shell, mejor seguridad
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/nunezlagos/domain"
LABEL org.opencontainers.image.description="Domain — memoria + orquestación AI agents"
LABEL org.opencontainers.image.licenses="proprietary"

COPY --from=builder /out/domain /usr/local/bin/domain
COPY --from=builder /out/domain-mcp /usr/local/bin/domain-mcp

USER nonroot:nonroot

EXPOSE 8000

# HEALTHCHECK definido en Dockerfile (ejecutado por docker engine, no k8s)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/domain", "healthcheck"]

ENTRYPOINT ["/usr/local/bin/domain"]
CMD ["server"]
