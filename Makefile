#
# Copyright (c) 2018 Cavium
#
# SPDX-License-Identifier: Apache-2.0
#


.PHONY: build test docker

VERSION=$(shell cat ./VERSION)
GOFLAGS=-ldflags "-X core-config-seed-go/main.Version=$(VERSION) -extldflags '-static'"

build:
	CGO_ENABLED=0 GOOS=linux go build -o core-config-seed-go $(GOFLAGS) -a main/main.go

test:
	go test ./...
	go vet ./...

prepare:
	go get github.com/hashicorp/consul/api
	go get github.com/magiconair/properties
	go get gopkg.in/yaml.v2

docker_core_config_seed_go:
	docker build \
			-f Dockerfile \
			-t edgexfoundry/docker-core-config-seed-go:latest \
			-t edgexfoundry/docker-core-config-seed-go:$(VERSION) \
			-t edgexfoundry/docker-core-config-seed-go:$(VERSION)-dev \
			.

docker_core_config_seed_go_arm:
	docker build \
			-f Dockerfile.aarch64 \
			-t edgexfoundry/docker-core-config-seed-go-arm:latest \
			-t edgexfoundry/docker-core-config-seed-go-arm:$(VERSION) \
			-t edgexfoundry/docker-core-config-seed-go-arm:$(VERSION)-dev \
			 .
