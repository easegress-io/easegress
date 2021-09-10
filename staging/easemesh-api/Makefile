# Copyright MegaEase Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SHELL:=/bin/bash
.PHONY: all check go json md html clean 

# Color
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
# no color 
NC='\033[0m'


# Path Related
MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
MKFILE_DIR := $(dir $(MKFILE_PATH))

RELEASE?=v1alpha1

# Protoc releated tools
PROTOC_GEN_DOC := $(GOPATH)/bin/protoc-gen-doc
PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go
PROTOC := $(shell which protoc)

# Detect the rely machine env 
UNAME := $(shell uname)

# Temporary generated golang model file and directory
TMP_GO_PACKAGE := ${MKFILE_DIR}${RELEASE}/easemesh
TMP_DIR := ${TMP_GO_PACKAGE}/api/${RELEASE}

# Target doc files 
MD_FILENAME := meshmodel.md
HTML_FILENAME := meshmodel.html
GO_FILENAME := meshmodel.pb.go
JSON_FILENAME := meshmodel.json

PROTO := $(shell find ${MKFILE_DIR}${RELEASE} -type f -name "*.proto") 

pre-check:
	@echo -e ${BLUE}"checking  dependent tool existing\n"${NC}
ifeq ($(PROTOC),)
# Run the right installation command for the Mac/Linux.
	ifeq ($(UNAME), Darwin)
		brew install protobuf
	endif
	ifeq ($(UNAME), Linux)
		sudo apt-get install protobuf-compiler
	endif
else 
	@echo -e ${BLUE}"protoc is already installed: $(PROTOC)\n"${NC}
endif 

# If $GOPATH/bin/protoc-gen-go/protoc-gen-doc does not exist, we'll run these commands to install
# them

ifeq ("$(wildcard $(PROTOC_GEN_GO))","")
	PROTOC_GEN_GO := $(shell which protoc-gen-go)
endif

ifeq ("$(wildcard $(PROTOC_GEN_GO))","")
	@echo -e ${BLUE}"install protoc-gen-go\n"${NC}
	go get -u github.com/golang/protobuf/protoc-gen-go	
else 
	@echo -e ${BLUE}"protoc-gen-go is already installed\n"${NC}
endif  

ifeq ("$(wildcard $(PROTOC_GEN_DOC))","")
	@echo -e ${BLUE}"install protoc-gen-doc\n"${NC}
	go get -u github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
else 
	@echo -e ${BLUE}"protoc-gen-doc is already installed\n"${NC}
endif  

go: ${PROTO} 
	@echo -e ${GREEN}"generate golang file from proto: ${PROTO}  \n"${NC}
	$(PROTOC) -I ${MKFILE_DIR}${RELEASE} --go_out=${MKFILE_DIR}${RELEASE}/ ${PROTO}  && \
	mv ${TMP_DIR}/*.go ${MKFILE_DIR}${RELEASE} && \
	rm -rf ${TMP_GO_PACKAGE}

md:  ${PROTO}
	@echo -e ${GREEN}"generate markdown file from proto: ${PROTO} \n"${NC}
	protoc -I ${MKFILE_DIR}${RELEASE} --doc_out=${MKFILE_DIR}${RELEASE}/ ${PROTO} --doc_opt=markdown,${MD_FILENAME}

html: ${PROTO}
	@echo -e ${GREEN}"generate html from proto: ${PROTO} \n"${NC}
	protoc -I ${MKFILE_DIR}${RELEASE} --doc_out=${MKFILE_DIR}${RELEASE}/ ${PROTO} --doc_opt=html,${HTML_FILENAME}

json: ${PROTO}
	@echo -e ${GREEN}"generate json from proto: ${PROTO} \n"${NC}
	protoc -I ${MKFILE_DIR}${RELEASE} --doc_out=${MKFILE_DIR}${RELEASE}/ ${PROTO} --doc_opt=json,${JSON_FILENAME}

all: pre-check go json md html

clean: 
	@echo -e ${RED}"clean temporary directory, and generated golang and doc files\n"${NC}
	rm -rf ${TMP_GO_PACKAGE} &&\
	rm ${MKFILE_DIR}${RELEASE}/${MD_FILENAME} && \
	rm ${MKFILE_DIR}${RELEASE}/${HTML_FILENAME} && \
	rm ${MKFILE_DIR}${RELEASE}/${GO_FILENAME} && \
	rm ${MKFILE_DIR}${RELEASE}/${JSON_FILENAME} 

