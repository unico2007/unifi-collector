# syntax=docker/dockerfile:1

# ---- Build stage ----------------------------------------------------------
FROM golang:1.25 AS builder

WORKDIR /src

# Cache dependencies first for faster incremental builds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static, stripped binary — no libc dependency, runnable on distroless/scratch.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/collector ./cmd/collector

# ---- Runtime stage --------------------------------------------------------
# distroless static: no shell, no package manager, runs as non-root (65532).
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/collector /collector

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/collector"]
CMD ["--config", "/etc/collector/config.yaml"]
