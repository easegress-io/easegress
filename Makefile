.PHONY: default build build_client build_server build_tool build_inventory run fmt clean \
		depend vendor_get vendor_update vendor_clean \
		build_client_alpine build_server_alpine build_server_ubuntu

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

GOPATH := ${MKFILE_DIR}
export GOPATH

RELEASE?=0.1.0
GIT_REPO_INFO=$(shell git config --get remote.origin.url)
DOCKER_REPO_INFO?=megaeasegateway/easegateway

ifndef COMMIT
  COMMIT := git-$(shell git rev-parse --short HEAD)
endif

DOCKER?=docker

GLIDE=${MKFILE_DIR}build/bin/glide

GATEWAY_ALL_SRC_FILES = $(shell find ${MKFILE_DIR}src -type f -name "*.go")
GATEWAY_CLIENT_SRC_FILES = $(shell find ${MKFILE_DIR}src/cli ${MKFILE_DIR}src/client -type f -name "*.go")
GATEWAY_SERVER_SRC_FILES = $(filter-out ${GATEWAY_CLIENT_SRC_FILES},${GATEWAY_ALL_SRC_FILES})
GATEWAY_INVENTORY_FILES=${MKFILE_DIR}src/inventory/*
GATEWAY_TOOL_SRC_FILES=$(shell find ${MKFILE_DIR}src/tool/ ${MKFILE_DIR}src/cluster -type f -name "*.go")

TARGET_GATEWAY_SERVER=${MKFILE_DIR}build/bin/easegateway-server
TARGET_GATEWAY_CLIENT=${MKFILE_DIR}build/bin/easegateway-client
TARGET_INVENTORY=${MKFILE_DIR}build/inventory
TARGET_GATEWAY_TOOL=${MKFILE_DIR}build/bin/easegateway-tool

TARGET=${TARGET_GATEWAY_SERVER} ${TARGET_GATEWAY_CLIENT} ${TARGET_GATEWAY_TOOL} ${TARGET_INVENTORY}

default: ${TARGET}

${TARGET_GATEWAY_TOOL} : ${GATEWAY_TOOL_SRC_FILES}
	@echo "-------------- building gateway tool ---------------"
	cd ${MKFILE_DIR} && \
		go build -i -v -ldflags "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${COMMIT} -X version.REPO=${GIT_REPO_INFO}" \
			-o ${TARGET_GATEWAY_TOOL} ${MKFILE_DIR}src/tool/main.go

${TARGET_GATEWAY_SERVER} : ${GATEWAY_SERVER_SRC_FILES}
	@echo "-------------- building gateway server ---------------"
	cd ${MKFILE_DIR} && \
		go build -i -v -ldflags "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${COMMIT} -X version.REPO=${GIT_REPO_INFO}" \
			-o ${TARGET_GATEWAY_SERVER} ${MKFILE_DIR}src/server/main.go

${TARGET_GATEWAY_CLIENT} : ${GATEWAY_CLIENT_SRC_FILES}
	@echo "-------------- building gateway client ---------------"
	cd ${MKFILE_DIR} && \
		go build -i -v -ldflags "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${COMMIT} -X version.REPO=${GIT_REPO_INFO}" \
			-o ${TARGET_GATEWAY_CLIENT} ${MKFILE_DIR}src/client/main.go

${TARGET_INVENTORY} : ${GATEWAY_INVENTORY_FILES}
	@echo "-------------- building inventory -------------"
	cd ${MKFILE_DIR} && rm -rf ${TARGET_INVENTORY} && mkdir -p ${TARGET_INVENTORY} && \
		cp -r ${GATEWAY_INVENTORY_FILES} ${TARGET_INVENTORY}

build: default

build_client: ${TARGET_GATEWAY_CLIENT}

build_server: ${TARGET_GATEWAY_SERVER}

build_tool: ${TARGET_GATEWAY_TOOL}

build_inventory: ${TARGET_INVENTORY}

clean:
	@rm -rf ${MKFILE_DIR}build && rm -rf ${MKFILE_DIR}rootfs/alpine/opt && rm -rf ${MKFILE_DIR}rootfs/ubuntu/opt
	@rm -rf ${MKFILE_DIR}pkg

run: build_server
	${TARGET_GATEWAY_SERVER}

fmt:
	cd ${MKFILE_DIR} && go fmt ./src/...

depend:
	GOBIN= GOPATH=${MKFILE_DIR}build go get -v github.com/Masterminds/glide

vendor_get: depend
	${GLIDE} install
	rm -rf ${MKFILE_DIR}src/vendor ${GLIDE} ${MKFILE_DIR}build/pkg ${MKFILE_DIR}build/src
	ln -s ${MKFILE_DIR}vendor ${MKFILE_DIR}src/vendor

vendor_update: depend
	rm -rf $(HOME)/.glide/cache
	${GLIDE} update && ${GLIDE} install
	rm -rf ${MKFILE_DIR}src/vendor ${GLIDE} ${MKFILE_DIR}build/pkg ${MKFILE_DIR}build/src
	ln -s ${MKFILE_DIR}vendor ${MKFILE_DIR}src/vendor

vendor_clean:
	rm -f ${MKFILE_DIR}glide.lock
	rm -dRf ${MKFILE_DIR}src/vendor ${MKFILE_DIR}vendor

build_client_alpine: ${TARGET_GATEWAY_CLIENT} ${TARGET_INVENTORY}
	@echo "-------------- building gateway client docker image (from alpine) ---------------"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
		mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && cp ${TARGET_GATEWAY_CLIENT} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
		cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/alpine/opt/easegateway && \
		cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:client-${RELEASE}_alpine -f ./Dockerfile.client .

build_server_alpine: ${TARGET_GATEWAY_SERVER} ${TARGET_INVENTORY}
	@echo "-------------- building gateway server docker image (from alpine) ---------------"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
		mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && cp ${TARGET_GATEWAY_SERVER} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
		cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/alpine/opt/easegateway && \
		cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_alpine -f ./Dockerfile.server .

build_server_ubuntu: ${TARGET_GATEWAY_SERVER} ${TARGET_INVENTORY}
	@echo "-------------- building gateway server docker image (from ubuntu) ---------------"
	rm -rf ${MKFILE_DIR}rootfs/ubuntu/opt && \
		mkdir -p ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && cp ${TARGET_GATEWAY_SERVER} ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && \
		cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway && \
		cd ${MKFILE_DIR}rootfs/ubuntu && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_ubuntu -f ./Dockerfile.server .

