###############################################################################
# Copyright 2017 Samsung Electronics All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
###############################################################################

# Docker image for building EdgeX Foundry Config Seed
FROM golang:1.8-alpine AS build-env

# environment variables
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$PATH

# download dependent go packages
RUN apk add --update git
RUN go get github.com/hashicorp/consul/api
RUN go get github.com/magiconair/properties
RUN go get gopkg.in/yaml.v2

# set the working directory
WORKDIR $GOPATH/src/github.com/edgexfoundry/core-config-seed-go

# copy go source files
COPY . .

# build
RUN apk update && apk add make
RUN make test
RUN make build

# Consul Docker image for EdgeX Foundry
FROM consul:0.7.3
RUN apk add --no-cache bash

# environment variables
ENV APP_DIR=/edgex/core-config-seed-go
ENV APP=core-config-seed-go
ENV WAIT_FOR_A_WHILE=5
ENV CONSUL_ARGS="-server -client=0.0.0.0 -bootstrap -ui"

# set the working directory
WORKDIR $APP_DIR

# copy files
COPY --from=build-env /go/src/github.com/edgexfoundry/core-config-seed-go/$APP .
COPY ./launch-consul-config.sh .
COPY ./docker-entrypoint.sh .
COPY ./res ./res
COPY ./config ./config

# call the wrapper to launch consul and config-seed application
CMD ["sh", "launch-consul-config.sh"]
