# SPDX-License-Identifier: BSD-3-Clause
#
# Authors: Alexander Jung <alex@nderjung.net>
#
# Copyright (c) 2020, Alexander Jung.  All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
# 1. Redistributions of source code must retain the above copyright
#    notice, this list of conditions and the following disclaimer.
# 2. Redistributions in binary form must reproduce the above copyright
#    notice, this list of conditions and the following disclaimer in the
#    documentation and/or other materials provided with the distribution.
# 3. Neither the name of the copyright holder nor the names of its
#    contributors may be used to endorse or promote products derived from
#    this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
# AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
# IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
# ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
# LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
# CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
# SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
# INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
# CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
# ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
# POSSIBILITY OF SUCH DAMAGE.

# Application configuration
BIN          ?= github-pr-comment
ORG          ?= nderjung
REPO         ?= concourse-github-pr-comment-resource
REGISTRY     ?= docker.io
GOOS         ?= linux
GOARCH       ?= amd64

# Directories and paths
WORKDIR      ?= $(CURDIR)
BUILDPATH    ?= $(WORKDIR)/dist/$(BIN)$(DIST)_$(GOOS)_$(GOARCH)

# Tools
DOCKER       ?= docker
GO           ?= go
GOFMT        ?= gofmt
GOLANGCILINT ?= golangci-lint
GORELEASER   ?= goreleaser
SUDO         ?= sudo

# Misc
Q            ?= @

# Targets
.PHONY: all
all: build

.PHONY: build
build: GOFLAGS ?= -linkshared
build:
	$(GO) build $(GOFLAGS) -o $(BUILDPATH)

# Build the docker container
docker: DOCKER_BUILD_EXTRA ?=
docker: DOCKER_TARGET      ?=
docker: DOCKER_TAG         ?=
ifeq ($(DOCKER_TARGET),)
docker: IMAGE_TAG          ?= latest
docker: DOCKER_TARGET      := run
else ifeq ($(DOCKER_TARGET),devenv)
docker: IMAGE_TAG          ?= dev
endif
.PHONY: docker
docker: GOLANG_VERSION     ?= 1.15
docker:
	$(Q)$(DOCKER) build \
		--tag ndrjng/$(REPO):$(IMAGE_TAG) \
		--file $(WORKDIR)/Dockerfile \
		--target $(DOCKER_TARGET) \
		--build-arg BIN=$(BIN) \
		--build-arg ORG=$(ORG) \
		--build-arg REPO=$(REPO) \
		--build-arg GOOS=$(GOOS) \
		--build-arg GOARCH=$(GOARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		$(DOCKER_BUILD_EXTRA) $(WORKDIR)

# Create a development environment
.PHONY: devenv
devenv: DOCKER_RUN_EXTRA ?=
devenv:
	$(Q)$(DOCKER) run -it --rm \
		--name $(BIN)-devenv \
		--workdir /go/src/github.com/$(ORG)/$(REPO) \
		--volume $(WORKDIR):/go/src/github.com/$(ORG)/$(REPO) \
		ndrjng/$(REPO):dev \
		$(DOCKER_RUN_EXTRA) bash

# CI/CD targets
.PHONY: ci-unit-test
ci-unit-test:
	$(GO) test -cover -v -race ./...

.PHONY: ci-static-analysis
ci-static-analysis:
	$(Q)$(GO) vet ./...
	$(Q)$(GOFMT) -s -l . 2>&1 | grep -vE '^\.git/' | grep -vE '^\.cache/'
	$(Q)$(GOLANGCILINT) run

.PHONY: ci-install-go-tools
ci-install-go-tools:
	$(Q)curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | $(SUDO) sh -s -- -b /usr/local/bin/ latest

.PHONY: ci-install-ci-tools
ci-install-ci-tools:
	$(Q)curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | $(SUDO) sh -s -- -b /usr/local/bin/ "v0.146.0"

.PHONY: ci-docker-login
ci-docker-login:
	$(Q)echo '${DOCKER_PASSWORD}' | docker login -u '${DOCKER_USERNAME}' --password-stdin '${REGISTRY}'

.PHONY: ci-docker-logout
ci-docker-logout:
	$(Q)$(DOCKER) logout '${REGISTRY}'

.PHONY: ci-publish-release
ci-publish-release:
	$(Q)$(GORELEASER) --rm-dist

.PHONY: ci-build-snapshot-packages
ci-build-snapshot-packages:
	$(Q)$(GORELEASER) \
		--snapshot \
		--skip-publish \
		--rm-dist

.PHONY: ci-release
ci-release:
	$(Q)$(GORELEASER) release --rm-dist

.PHONY: ci-test-production-image
ci-test-production-image:
	$(Q)$(DOCKER) run --rm -t \
		${REGISTRY}/ndrjng/$(REPO):latest \
			/bin/$(BIN) --version

.PHONY: ci-test-linux-run
ci-test-linux-run:
	chmod +x $(BUILDPATH)
	$(BUILDPATH) --version
