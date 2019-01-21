.PHONY: build build_client build_server \
		build_client_alpine build_server_alpine build_server_ubuntu \
		run fmt vet clean \
		mod_update vendor_from_mod vendor_clean

export GO111MODULE=on

# Path Related
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

# Version
RELEASE?=2.0.0
# Git Related
GIT_REPO_INFO=$(shell cd ${MKFILE_DIR} && git config --get remote.origin.url)
ifndef GIT_COMMIT
  GIT_COMMIT := git-$(shell git rev-parse --short HEAD)
endif

# Docker Related
DOCKER=docker
DOCKER_REPO_INFO?=megaeasegateway/easegateway

# Source Files
ALL_FILES = $(shell find ${MKFILE_DIR}{cmd,pkg} -type f -name "*.go")
CLIENT_FILES = $(shell find ${MKFILE_DIR}{cmd/client,pkg} -type f -name "*.go")
SERVER_FILES = $(shell find ${MKFILE_DIR}{cmd/server,pkg} -type f -name "*.go")

# Targets
TARGET_SERVER=${MKFILE_DIR}bin/easegateway-server
TARGET_CLIENT=${MKFILE_DIR}bin/egwctl
TARGET=${TARGET_SERVER} ${TARGET_CLIENT}

# Rules
build: ${TARGET}

build_client: ${TARGET_CLIENT}

build_server: ${TARGET_SERVER}

build_client_alpine: ${TARGET_CLIENT}
	@echo "build client docker image (from alpine)"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp ${TARGET_CLIENT} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:client-${RELEASE}_alpine -f ./Dockerfile.client .

build_server_alpine: ${TARGET_SERVER}
	@echo "build server docker image (from alpine)"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp ${TARGET_SERVER} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_alpine -f ./Dockerfile.server .

build_server_ubuntu: ${TARGET_SERVER}
	@echo "build server docker image (from ubuntu)"
	rm -rf ${MKFILE_DIR}rootfs/ubuntu/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && \
	cp ${TARGET_SERVER} ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && \
	cd ${MKFILE_DIR}rootfs/ubuntu && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_ubuntu -f ./Dockerfile.server .

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


GO_LD_FLAGS= "-s -w -X github.com/megaease/easegateway/pkg/version.RELEASE=${RELEASE} -X github.com/megaease/easegateway/pkg/version.COMMIT=${GIT_COMMIT} -X github.com/megaease/easegateway/pkg/version.REPO=${GIT_REPO_INFO}"
${TARGET_SERVER} : ${SERVER_FILES}
	@echo "build server"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -i -v -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_SERVER} ${MKFILE_DIR}cmd/server/main.go

${TARGET_CLIENT} : ${CLIENT_FILES}
	@echo "build client"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -i -v -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_CLIENT} ${MKFILE_DIR}cmd/client/main.go

${TARGET_INVENTORY} : ${INVENTORY_FILES}
	@echo "build inventory"
	cd ${MKFILE_DIR} && \
	rm -rf ${TARGET_INVENTORY} && \
	mkdir -p ${TARGET_INVENTORY} && \
	cp -r ${INVENTORY_FILES} ${TARGET_INVENTORY}
