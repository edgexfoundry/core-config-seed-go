#
# Copyright (c) 2018 Cavium
#
# SPDX-License-Identifier: Apache-2.0
#


.PHONY: build test docker

VERSION=$(shell cat ./VERSION)
GOFLAGS=-ldflags "-X github.com/edgexfoundry/core-config-seed-go.Version=$(VERSION) -extldflags '-static'"
GIT_SHA=$(shell git rev-parse HEAD)
build:
	CGO_ENABLED=0 go build -o core-config-seed-go $(GOFLAGS) -a main.go

test:
	go test -cover ./...
	go vet ./...

prepare:
	glide install

docker: docker_core_config_seed_go

docker_core_config_seed_go:
	docker build \
			-f Dockerfile \
			--label "git_sha=$(GIT_SHA)" \
			-t edgexfoundry/docker-core-config-seed-go:$(GIT_SHA) \
			-t edgexfoundry/docker-core-config-seed-go:$(VERSION)-dev \
			.
docker_core_config_seed_go_arm:
	docker build \
			-f Dockerfile.aarch64 \
			--label "git_sha=$(GIT_SHA)" \
			-t edgexfoundry/docker-core-config-seed-go:$(GIT_SHA) \
			-t edgexfoundry/docker-core-config-seed-go:$(VERSION)-dev \
			.

