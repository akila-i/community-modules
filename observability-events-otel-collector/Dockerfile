# Copyright 2026 The OpenChoreo Authors
# SPDX-License-Identifier: Apache-2.0

# syntax=docker/dockerfile:1

# ── Stage 1: build the custom collector distribution with OCB ──
# Pinned to the OTel Collector v0.153.0. client-go v0.36 (Kubernetes 1.36)
# requires Go 1.26+. Pinned to $BUILDPLATFORM and cross-compiled to $TARGETARCH
# so multi-arch builds run natively instead of under slow QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src

ARG OCB_VERSION=v0.153.0
RUN go install go.opentelemetry.io/collector/cmd/builder@${OCB_VERSION}

# Bring in the manifest and the local processor source.
COPY builder-config.yaml .
COPY k8seventenrichprocessor/ ./k8seventenrichprocessor/

# Compile. CGO is off (pure-Go deps) so the binary is static and cross-compiles
# cleanly. Output lands in ./dist (per builder-config.yaml output_path).
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    builder --config=builder-config.yaml

# ── Stage 2: minimal, non-root runtime ──
# distroless/static:nonroot ships CA certificates and runs as uid 65532 with no
# shell or package manager — small image, small attack surface.
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /src/dist/otelcol-openchoreo-events /otelcol-openchoreo-events

# OTLP ingest (if that pipeline is enabled) + health_check probe endpoint.
EXPOSE 4317 4318 13133

ENTRYPOINT ["/otelcol-openchoreo-events"]
CMD ["--config", "/etc/otelcol/config.yaml"]
