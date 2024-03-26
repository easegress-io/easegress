SHELL:=/bin/sh
.PHONY: build build_client build_server build_docker \
		test run fmt vet clean \
		mod_update vendor_from_mod vendor_clean

export GO111MODULE=on

# Path Related
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))
RELEASE_DIR := ${MKFILE_DIR}bin
GO_PATH := $(shell go env | grep GOPATH | awk -F "=" '{print $$2}' | tr -d "'")
INTEGRATION_TEST_PATH := build/test

# Image Name
IMAGE_NAME?=megaease/easegress
BUILDER_IMAGE_NAME?=megaease/golang:1.21.7-alpine

# Version
RELEASE?=v2.7.3

# Git Related
GIT_REPO_INFO=$(shell cd ${MKFILE_DIR} && git config --get remote.origin.url)
ifndef GIT_COMMIT
  GIT_COMMIT := git-$(shell git rev-parse --short HEAD)
endif

# Build Flags
GO_LD_FLAGS= "-s -w -X github.com/megaease/easegress/v2/pkg/version.RELEASE=${RELEASE} -X github.com/megaease/easegress/v2/pkg/version.COMMIT=${GIT_COMMIT} -X github.com/megaease/easegress/v2/pkg/version.REPO=${GIT_REPO_INFO}"

# Cgo is disabled by default
ENABLE_CGO= CGO_ENABLED=0

# Check Go build tags, the tags are from command line of make
ifdef GOTAGS
  GO_BUILD_TAGS= -tags ${GOTAGS}
  # Must enable Cgo when wasmhost is included
  ifeq ($(findstring wasmhost,${GOTAGS}), wasmhost)
	ENABLE_CGO= CGO_ENABLED=1
  endif
endif

# When build binaries for docker, we put the binaries to another folder to avoid
# overwriting existing build result, or Mac/Windows user will have to do a rebuild
# after build the docker image, which is Linux only currently.
ifdef DOCKER
  RELEASE_DIR= ${MKFILE_DIR}build/bin
endif

# Targets
TARGET_SERVER=${RELEASE_DIR}/easegress-server
TARGET_CLIENT=${RELEASE_DIR}/egctl
TARGET_BUILDER=${RELEASE_DIR}/egbuilder

# Rules
build: build_client build_server build_builder

wasm: ENABLE_CGO=CGO_ENABLED=1
wasm: GO_BUILD_TAGS=-tags wasmhost
wasm: build

build_client:
	@echo "build client"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -v -trimpath -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_CLIENT} ${MKFILE_DIR}cmd/client

build_builder:
	@echo "build builder"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -v -trimpath -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_BUILDER} ${MKFILE_DIR}cmd/builder

build_server:
	@echo "build server"
	cd ${MKFILE_DIR} && \
	${ENABLE_CGO} go build ${GO_BUILD_TAGS} -v -trimpath -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_SERVER} ${MKFILE_DIR}cmd/server

dev_build: dev_build_client dev_build_server

dev_build_client:
	@echo "build dev client"
	cd ${MKFILE_DIR} && \
	go build -v -race -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_CLIENT} ${MKFILE_DIR}cmd/client

dev_build_server:
	@echo "build dev server"
	cd ${MKFILE_DIR} && \
	go build -v -race -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_SERVER} ${MKFILE_DIR}cmd/server

build_docker:
	cd ${MKFILE_DIR}
	mkdir -p build/cache
	mkdir -p build/bin
	docker run -w /egsrc --rm \
	-v ${GO_PATH}:/gopath -v ${MKFILE_DIR}:/egsrc -v ${MKFILE_DIR}build/cache:/gocache \
	-e GOPROXY=https://goproxy.io,direct -e GOCACHE=/gocache -e GOPATH=/gopath \
	${BUILDER_IMAGE_NAME} make build DOCKER=true
	docker buildx build --platform linux/amd64 --load -t ${IMAGE_NAME}:${RELEASE} -f ./build/package/Dockerfile .
	docker tag ${IMAGE_NAME}:${RELEASE} ${IMAGE_NAME}:latest
	docker tag ${IMAGE_NAME}:latest ${IMAGE_NAME}:server-sidecar
	docker tag ${IMAGE_NAME}:latest ${IMAGE_NAME}:easemesh

build_golang_docker:
	docker buildx build \
	--platform linux/amd64 \
	-t ${BUILDER_IMAGE_NAME} \
	-f build/package/Dockerfile.builder .

test:
	cd ${MKFILE_DIR}
	go mod tidy
	git diff --exit-code go.mod go.sum
	go mod verify
	go test -v -gcflags "all=-l" ${MKFILE_DIR}pkg/... ${MKFILE_DIR}cmd/... ${TEST_FLAGS}

integration_test: build
	{ \
	set -e ;\
	cd ${INTEGRATION_TEST_PATH} ;\
	./test.sh ;\
	./clean.sh ;\
    }

clean:
	rm -rf ${RELEASE_DIR}
	rm -rf ${MKFILE_DIR}build/cache
	rm -rf ${MKFILE_DIR}build/bin

run: build_server

fmt:
	cd ${MKFILE_DIR} && go fmt ./...

vet:
	cd ${MKFILE_DIR} && go vet ./...

vendor_from_mod:
	cd ${MKFILE_DIR} && go mod vendor

vendor_clean:
	rm -rf ${MKFILE_DIR}vendor

mod_update:
	cd ${MKFILE_DIR} && go get -u
