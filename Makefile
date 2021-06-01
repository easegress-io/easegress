SHELL:=/bin/sh
.PHONY: build build_client build_server build_docker \
		test run fmt vet clean \
		mod_update vendor_from_mod vendor_clean

export GO111MODULE=on
export GOPROXY=https://goproxy.io

# Path Related
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

# Version
RELEASE?=1.0.0

# Git Related
GIT_REPO_INFO=$(shell cd ${MKFILE_DIR} && git config --get remote.origin.url)
ifndef GIT_COMMIT
  GIT_COMMIT := git-$(shell git rev-parse --short HEAD)
endif

# Source Files
ALL_FILES = $(shell find ${MKFILE_DIR}${cmd,pkg} -type f -name "*.go")
CLIENT_FILES = $(shell find ${MKFILE_DIR}${cmd/client,pkg} -type f -name "*.go")
SERVER_FILES = $(shell find ${MKFILE_DIR}${cmd/server,pkg} -type f -name "*.go")

# Targets
TARGET_SERVER=${MKFILE_DIR}bin/easegress-server
TARGET_CLIENT=${MKFILE_DIR}bin/egctl
TARGET=${TARGET_SERVER} ${TARGET_CLIENT}

# Rules
build: ${TARGET}

build_client: ${TARGET_CLIENT}

build_server: ${TARGET_SERVER}

build_docker:
	docker build -t megaease/easegress:${RELEASE} -f ./build/package/Dockerfile .

test:
	@go list ./{cmd,pkg}/... | grep -v -E 'vendor' | xargs -n1 go test

clean:
	rm -rf ${TARGET}

run: build_server
	${TARGET_SERVER}

fmt:
	cd ${MKFILE_DIR} && go fmt ./{cmd,pkg}/...

vet:
	cd ${MKFILE_DIR} && go vet ./{cmd,pkg}/...

vendor_from_mod:
	cd ${MKFILE_DIR} && go mod vendor

vendor_clean:
	rm -rf ${MKFILE_DIR}vendor

mod_update:
	cd ${MKFILE_DIR} && go get -u


GO_LD_FLAGS= "-s -w -X github.com/megaease/easegress/pkg/version.RELEASE=${RELEASE} -X github.com/megaease/easegress/pkg/version.COMMIT=${GIT_COMMIT} -X github.com/megaease/easegress/pkg/version.REPO=${GIT_REPO_INFO}"
${TARGET_SERVER} : ${SERVER_FILES}
	@echo "build server"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -v -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_SERVER} ${MKFILE_DIR}cmd/server/main.go

${TARGET_CLIENT} : ${CLIENT_FILES}
	@echo "build client"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -v -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_CLIENT} ${MKFILE_DIR}cmd/client/main.go