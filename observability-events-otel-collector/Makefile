# Copyright 2026 The OpenChoreo Authors
# SPDX-License-Identifier: Apache-2.0

# The custom processor lives in a nested Go module, so test targets descend into
# it. The collector binary itself is produced by the OpenTelemetry Collector
# Builder (OCB) at image-build time (see Dockerfile / builder-config.yaml).

MODULE_NAME := $(notdir $(CURDIR))
PROCESSOR_DIR := k8seventenrichprocessor
OCB_VERSION ?= v0.153.0

# Container build. Override DOCKER to use a drop-in (e.g. a nerdctl wrapper):
#   make docker-build DOCKER=~/nerdctl-docker-wrapper.sh
DOCKER ?= docker
IMAGE ?= observability-events-otel-collector:dev

.PHONY: unit-test test vet tidy build docker-build clean

# Required by CI: runs unit tests and leaves the coverage profile at the repo
# root as <module>-coverage.out for the Codecov upload step.
unit-test:
	cd $(PROCESSOR_DIR) && go test -coverprofile=coverage.out -covermode=atomic ./...
	mv $(PROCESSOR_DIR)/coverage.out ../$(MODULE_NAME)-coverage.out

# Local convenience.
test:
	cd $(PROCESSOR_DIR) && go test -race ./...

vet:
	cd $(PROCESSOR_DIR) && go vet ./...

tidy:
	cd $(PROCESSOR_DIR) && go mod tidy

# Build the collector distribution locally with OCB (output in ./dist).
build:
	go install go.opentelemetry.io/collector/cmd/builder@$(OCB_VERSION)
	$(shell go env GOPATH)/bin/builder --config=builder-config.yaml

# Build the container image (OCB runs inside the multi-stage Dockerfile).
docker-build:
	$(DOCKER) build -t $(IMAGE) -f Dockerfile .

clean:
	rm -rf dist
