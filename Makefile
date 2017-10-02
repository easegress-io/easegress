.PHONY: default build build_client build_server build_inventory \
		run fmt vendor_clean vendor_get vendor_update clean \
		build_client_alpine build_server_alpine \
		build_server_ubuntu

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

GOPATH := ${MKFILE_DIR}_vendor:${MKFILE_DIR}
export GOPATH

RELEASE?=0.1.0
GIT_REPO_INFO=$(shell git config --get remote.origin.url)
DOCKER_REPO_INFO?=megaeasegateway/easegateway

ifndef COMMIT
  COMMIT := git-$(shell git rev-parse --short HEAD)
endif

DOCKER?=docker

GATEWAY_ALL_SRC_FILES = $(shell find ${MKFILE_DIR}src -type f -name "*.go")
GATEWAY_CLIENT_SRC_FILES = $(shell find ${MKFILE_DIR}src/cli ${MKFILE_DIR}src/client -type f -name "*.go")
GATEWAY_SERVER_SRC_FILES = $(filter-out ${GATEWAY_CLIENT_SRC_FILES},${GATEWAY_ALL_SRC_FILES})
GATEWAY_INVENTORY_FILES=${MKFILE_DIR}src/inventory/*

TARGET_GATEWAY_SERVER=${MKFILE_DIR}build/bin/easegateway-server
TARGET_GATEWAY_CLIENT=${MKFILE_DIR}build/bin/easegateway-client
TARGET_INVENTORY=${MKFILE_DIR}build/inventory

TARGET=${TARGET_GATEWAY_SERVER} ${TARGET_GATEWAY_CLIENT} ${TARGET_INVENTORY}

default: ${TARGET}

${TARGET_GATEWAY_SERVER} : ${GATEWAY_SERVER_SRC_FILES}
	@echo "-------------- building gateway server ---------------"
	cd ${MKFILE_DIR} && \
		go build -v -ldflags "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${COMMIT} -X version.REPO=${GIT_REPO_INFO}" \
			-o ${TARGET_GATEWAY_SERVER} ${MKFILE_DIR}src/server/main.go

${TARGET_GATEWAY_CLIENT} : ${GATEWAY_CLIENT_SRC_FILES}
	@echo "-------------- building gateway client ---------------"
	cd ${MKFILE_DIR} && \
		go build -v -ldflags "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${COMMIT} -X version.REPO=${GIT_REPO_INFO}" \
			-o ${TARGET_GATEWAY_CLIENT} ${MKFILE_DIR}src/client/main.go

${TARGET_INVENTORY} : ${GATEWAY_INVENTORY_FILES}
	@echo "-------------- building inventory -------------"
	cd ${MKFILE_DIR} && rm -rf ${TARGET_INVENTORY} && mkdir -p ${TARGET_INVENTORY} && \
		cp -r ${GATEWAY_INVENTORY_FILES} ${TARGET_INVENTORY}

build: default

build_client: ${TARGET_GATEWAY_CLIENT}

build_server: ${TARGET_GATEWAY_SERVER}

build_inventory: ${TARGET_INVENTORY}

run: build_server
	${TARGET_GATEWAY_SERVER} -host=localhost -certfile=localhost-cert.pem -keyfile=localhost-key.pem

fmt:
	cd ${MKFILE_DIR} && go fmt ./src/...

vendor_clean:
	rm -dRf ${MKFILE_DIR}_vendor/src

vendor_get:
	GOPATH=${MKFILE_DIR}_vendor go get -d -u -v \
		github.com/sirupsen/logrus \
		github.com/Shopify/sarama \
		github.com/bsm/sarama-cluster \
		github.com/xeipuuv/gojsonschema \
		github.com/ant0ine/go-json-rest/rest \
		github.com/rcrowley/go-metrics \
		golang.org/x/time/rate \
		github.com/urfave/cli \
		github.com/hashicorp/memberlist \
		github.com/dgraph-io/badger \
		github.com/ugorji/go/codec \
		github.com/hashicorp/logutils \
		github.com/ghodss/yaml \
		github.com/hexdecteam/easegateway-types/... \
		github.com/hexdecteam/easegateway-go-client/...

vendor_update: vendor_get
	cd ${MKFILE_DIR} && rm -rf `find ./_vendor/src -type d -name .git` \
	&& rm -rf `find ./_vendor/src -type d -name .hg` \
	&& rm -rf `find ./_vendor/src -type d -name .bzr` \
	&& rm -rf `find ./_vendor/src -type d -name .svn`

clean:
	@rm -rf ${MKFILE_DIR}build && rm -rf ${MKFILE_DIR}rootfs/alpine/opt && rm -rf ${MKFILE_DIR}rootfs/ubuntu/opt

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

