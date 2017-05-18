.PHONY: default build build_client build_server build_inventory \
		run fmt vendor_clean vendor_get vendor_update clean

MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

GOPATH := ${MKFILE_DIR}_vendor:${MKFILE_DIR}
export GOPATH

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
	cd ${MKFILE_DIR} && go build  -gcflags "-N -l"  -v -o ${TARGET_GATEWAY_SERVER} ${MKFILE_DIR}src/server/main.go

${TARGET_GATEWAY_CLIENT} : ${GATEWAY_CLIENT_SRC_FILES}
	@echo "-------------- building gateway client ---------------"
	cd ${MKFILE_DIR} && go build  -gcflags "-N -l"  -v -o ${TARGET_GATEWAY_CLIENT} ${MKFILE_DIR}src/client/main.go

${TARGET_INVENTORY} : ${GATEWAY_INVENTORY_FILES}
	@echo "-------------- building inventory -------------"
	cd ${MKFILE_DIR} && rm -rf ${TARGET_INVENTORY} && mkdir -p ${TARGET_INVENTORY} && cp -r ${GATEWAY_INVENTORY_FILES} ${TARGET_INVENTORY}

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
		github.com/golang/protobuf/proto \
		github.com/rcrowley/go-metrics \
		golang.org/x/time/rate \
		github.com/urfave/cli \
		github.com/hashicorp/memberlist \
		github.com/hexdecteam/easegateway-go-client/...

vendor_update: vendor_get
	cd ${MKFILE_DIR} && rm -rf `find ./_vendor/src -type d -name .git` \
	&& rm -rf `find ./_vendor/src -type d -name .hg` \
	&& rm -rf `find ./_vendor/src -type d -name .bzr` \
	&& rm -rf `find ./_vendor/src -type d -name .svn`

clean:
	@rm -rf ${MKFILE_DIR}build
