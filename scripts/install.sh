#!/bin/bash

# First - check OS.
OS="$(uname)"
if [[ "${OS}" == "Linux" ]]; then
    OS=linux
elif [[ "${OS}" == "Darwin" ]];then
    OS=darwin
else
    abort "Unsupport OS - ${OS}"
fi

#Second - check the CPU arch
#refer to: https://stackoverflow.com/questions/45125516/possible-values-for-uname-m
ARCH=$(uname -m)
if [[ $ARCH == x86_64 ]]; then
    ARCH=amd64
elif  [[ $ARCH == i686 || $ARCH == i386 ]]; then
    ARCH=386
elif  [[ $ARCH == aarch64* || $ARCH == armv8* ]]; then
    ARCH=arm64
else
    abort "Unsupport CPU - ${ARCH}"
fi

#Third - download the binaries
GITHUB_URL=https://github.com/megaease/easegress
LATEST_RELEASE=$(curl -L -s -H 'Accept: application/json' ${GITHUB_URL}/releases/latest)
LATEST_VERSION=$(echo $LATEST_RELEASE | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
ARTIFACT="easegress-${LATEST_VERSION}-${OS}-${ARCH}.tar.gz"
ARTIFACT_URL="${GITHUB_URL}/releases/download/${LATEST_VERSION}/${ARTIFACT}"

mkdir -p ./easegress
curl -L ${ARTIFACT_URL} -o ./easegress/${ARTIFACT}
tar -zxf ./easegress/${ARTIFACT} -C easegress 