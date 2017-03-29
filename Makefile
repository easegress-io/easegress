.PHONY: default build build_cmd build_gateway build_inventory \
		run fmt vendor_clean vendor_get vendor_update clean

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

GOPATH := ${MKFILE_DIR}_vendor:${MKFILE_DIR}
export GOPATH

GATEWAY_ALL_SRC_FILES = $(shell find ${MKFILE_DIR}src -type f -name "*.go")
GATEWAY_CMD_SRC_FILES = $(shell find ${MKFILE_DIR}src/cmd -type f -name "*.go")
GATEWAY_SRC_FILES = $(filter-out ${GATEWAY_CMD_SRC_FILES},${GATEWAY_ALL_SRC_FILES})
GATEWAY_INVENTORY_FILES=${MKFILE_DIR}src/inventory/*

TARGET_GATEWAY_CMD=${MKFILE_DIR}build/bin/easegateway_admin
TARGET_GATEWAY=${MKFILE_DIR}build/bin/easegateway
TARGET_INVENTORY=${MKFILE_DIR}build/inventory

TARGET=${TARGET_GATEWAY_CMD} ${TARGET_GATEWAY} ${TARGET_INVENTORY}

default: ${TARGET}

${TARGET_GATEWAY_CMD} : ${GATEWAY_CMD_SRC_FILES}
	@echo "-------------- building gateway cmd------------"
	cd ${MKFILE_DIR} && go build  -gcflags "-N -l"  -v -o ${TARGET_GATEWAY_CMD} ${GATEWAY_CMD_SRC_FILES}

${TARGET_GATEWAY} : ${GATEWAY_SRC_FILES}
	@echo "-------------- building gateway ---------------"
	cd ${MKFILE_DIR} && go build  -gcflags "-N -l"  -v -o ${TARGET_GATEWAY} ${MKFILE_DIR}src/main.go

${TARGET_INVENTORY} : ${GATEWAY_INVENTORY_FILES}
	@echo "-------------- building inventory -------------"
	cd ${MKFILE_DIR} && rm -rf ${TARGET_INVENTORY} && mkdir -p ${TARGET_INVENTORY} && cp -r ${GATEWAY_INVENTORY_FILES} ${TARGET_INVENTORY}

build: default

build_cmd:

build_gateway: ${TARGET_GATEWAY}

build_inventory: ${TARGET_INVENTORY}

run: build_gateway
	${TARGET_GATEWAY}

fmt:
	cd ${MKFILE_DIR} && go fmt ./src/...

vendor_clean:
	rm -dRf ${MKFILE_DIR}_vendor/src

vendor_get:
	GOPATH=${MKFILE_DIR}_vendor go get -d -u -v \
		github.com/sirupsen/logrus \
		github.com/Shopify/sarama \
		github.com/bsm/sarama-cluster \
		github.com/BurntSushi/toml \
		github.com/xeipuuv/gojsonschema \
		github.com/ant0ine/go-json-rest/rest \
		github.com/golang/protobuf/proto \
		github.com/rcrowley/go-metrics \
		golang.org/x/time/rate \
		github.com/urfave/cli

vendor_update: vendor_get
	cd ${MKFILE_DIR} && rm -rf `find ./_vendor/src -type d -name .git` \
	&& rm -rf `find ./_vendor/src -type d -name .hg` \
	&& rm -rf `find ./_vendor/src -type d -name .bzr` \
	&& rm -rf `find ./_vendor/src -type d -name .svn`

clean:
	@rm -rf ${MKFILE_DIR}build
