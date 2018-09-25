.PHONY: default build build_client build_server build_tool build_inventory \
		build_client_alpine build_server_alpine build_server_ubuntu \
		run fmt vet clean \
		depend vendor_get vendor_update vendor_clean

# Path Related
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

# Git Related
GIT_REPO_INFO=$(shell cd ${MKFILE_DIR} && git config --get remote.origin.url)
ifndef GIT_COMMIT
  GIT_COMMIT := git-$(shell git rev-parse --short HEAD)
endif

# Docker Related
DOCKER=docker
DOCKER_REPO_INFO?=megaeasegateway/easegateway

# MISC
RELEASE?=0.1.0

GLIDE=${GOPATH}/bin/glide

# Source Files
ALL_FILES = $(shell find ${MKFILE_DIR}{cmd,pkg} -type f -name "*.go")
CLIENT_FILES = $(shell find ${MKFILE_DIR}{cmd/client,pkg/cli,pkg/common} -type f -name "*.go")
SERVER_FILES = $(shell find ${MKFILE_DIR}{cmd/server,pkg} -type f -name "*.go")
TOOL_FILES = $(shell find ${MKFILE_DIR}{cmd/tool,pkg} -type f -name "*.go")
INVENTORY_FILES=${MKFILE_DIR}inventory/*

# Targets
TARGET_SERVER=${GOPATH}/bin/easegateway-server
TARGET_CLIENT=${GOPATH}/bin/easegateway-client
TARGET_TOOL=${GOPATH}/bin/easegateway-tool
TARGET_INVENTORY=${GOPATH}/inventory
TARGET=${TARGET_SERVER} ${TARGET_CLIENT} ${TARGET_TOOL} ${TARGET_INVENTORY}


# Rules
default: ${TARGET}

build: default

build_client: ${TARGET_CLIENT}

build_server: ${TARGET_SERVER}

build_tool: ${TARGET_TOOL}

build_inventory: ${TARGET_INVENTORY}

build_client_alpine: ${TARGET_CLIENT} ${TARGET_INVENTORY}
	@echo "build client docker image (from alpine)"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp ${TARGET_CLIENT} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/alpine/opt/easegateway && \
	cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:client-${RELEASE}_alpine -f ./Dockerfile.client .

build_server_alpine: ${TARGET_SERVER} ${TARGET_INVENTORY}
	@echo "build server docker image (from alpine)"
	rm -rf ${MKFILE_DIR}rootfs/alpine/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp ${TARGET_SERVER} ${MKFILE_DIR}rootfs/alpine/opt/easegateway/bin && \
	cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/alpine/opt/easegateway && \
	cd ${MKFILE_DIR}rootfs/alpine && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_alpine -f ./Dockerfile.server .

build_server_ubuntu: ${TARGET_SERVER} ${TARGET_INVENTORY}
	@echo "build server docker image (from ubuntu)"
	rm -rf ${MKFILE_DIR}rootfs/ubuntu/opt && \
	mkdir -p ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && \
	cp ${TARGET_SERVER} ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway/bin && \
	cp -r ${TARGET_INVENTORY} ${MKFILE_DIR}rootfs/ubuntu/opt/easegateway && \
	cd ${MKFILE_DIR}rootfs/ubuntu && $(DOCKER) build -t ${DOCKER_REPO_INFO}:server-${RELEASE}_ubuntu -f ./Dockerfile.server .

# subnet sets up the require subnet for testing on darwin (osx) - you must run
# this before running other tests if you are on osx.
subnet::
	@sh -c "'${MKFILE_DIR}/scripts/setup_test_subnet.sh'"

## TODO (shengdong, zhiyan) make xargs exit when go test failed, tune @go list ./{cmd,pkg}/... | grep -v -E 'vendor' | xargs -n1 sh -c 'GOPATH=${GOPATH} go test "$@"|| exit 255'
quick_test:: subnet
	@go list ./{cmd,pkg}/... | grep -v -E 'vendor' | xargs -n1 go test -short

test:: subnet
	@go list ./{cmd,pkg}/... | grep -v -E 'vendor' | xargs -n1 go test

clean:
	rm -rf ${TARGET}

run: build_server
	${TARGET_SERVER}

fmt:
	cd ${MKFILE_DIR} && go fmt ./{cmd,pkg}/...

vet:
	cd ${MKFILE_DIR} && go vet ./{cmd,pkg}/...

depend: ${GLIDE}

vendor_get: depend
	cd ${MKFILE_DIR} && ${GLIDE} install

vendor_update: depend
	cd ${MKFILE_DIR} && ${GLIDE} update

vendor_clean:
	rm -rf ${MKFILE_DIR}glide.lock ${MKFILE_DIR}vendor

${GLIDE} :
	go get -v github.com/Masterminds/glide

GO_LD_FLAGS= "-s -w -X version.RELEASE=${RELEASE} -X version.COMMIT=${GIT_COMMIT} -X version.REPO=${GIT_REPO_INFO}"
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

${TARGET_TOOL} : ${TOOL_FILES}
	@echo "build tool"
	cd ${MKFILE_DIR} && \
	CGO_ENABLED=0 go build -i -v -ldflags ${GO_LD_FLAGS} \
	-o ${TARGET_TOOL} ${MKFILE_DIR}cmd/tool/main.go

${TARGET_INVENTORY} : ${INVENTORY_FILES}
	@echo "build inventory"
	cd ${MKFILE_DIR} && \
	rm -rf ${TARGET_INVENTORY} && \
	mkdir -p ${TARGET_INVENTORY} && \
	cp -r ${INVENTORY_FILES} ${TARGET_INVENTORY}
