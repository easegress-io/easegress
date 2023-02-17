#!/bin/bash

set -e

RED='\033[0;31m'
NC='\033[0m'

# First - check OS.
OS="$(uname)"
if [[ "${OS}" == "Linux" ]]; then
    OS=linux
    DISTRO=$(awk -F= '/^NAME/{print $2}' /etc/os-release | tr -d '\"')
elif [[ "${OS}" == "Darwin" ]];then
    OS=darwin
else
    echo -e "Error: ${RED}Unsupport OS - ${OS}${NC}"
    exit
fi

# Second - check the CPU arch
# refer to: https://stackoverflow.com/questions/45125516/possible-values-for-uname-m
ARCH=$(uname -m)
if [[ $ARCH == x86_64 ]]; then
    ARCH=amd64
elif  [[ $ARCH == i686 || $ARCH == i386 ]]; then
    ARCH=386
elif  [[ $ARCH == aarch64* || $ARCH == armv8* || $ARCH == arm64* ]]; then
    ARCH=arm64
else
    echo -e "Error: ${RED}Unsupport CPU - ${ARCH}${NC}"
    exit
fi

# Third - download the binaries
GITHUB_URL=https://github.com/megaease/easegress
LATEST_RELEASE=$(curl -L -s -H 'Accept: application/json' ${GITHUB_URL}/releases/latest)
LATEST_VERSION=$(echo $LATEST_RELEASE | sed -e 's/.*"tag_name":"\([^"]*\)".*/\1/')
ARTIFACT="easegress-${LATEST_VERSION}-${OS}-${ARCH}.tar.gz"
ARTIFACT_URL="${GITHUB_URL}/releases/download/${LATEST_VERSION}/${ARTIFACT}"


read -p "Enter the install directory [$(pwd)/easegress]: " DIR
DIR=${DIR:-$(pwd)/easegress}
BINDIR=${DIR}/bin

mkdir -p ${DIR}
echo "Create the directory - \"${DIR}\" successfully."
echo "Downloading the release file - \"${ARTIFACT}\" ..."
curl -sL ${ARTIFACT_URL} -o ${DIR}/${ARTIFACT}
echo "Downloaded \"${ARTIFACT}\""
tar -zxf ${DIR}/${ARTIFACT} -C "${DIR}"
echo "Extract the files successfully"

# Fourth - configure the easegress
echo "Download the config.yaml file"
RAW_GITHUB_URL=https://raw.githubusercontent.com/megaease/easegress
curl -sL ${RAW_GITHUB_URL}/main/scripts/config.yaml -o ${DIR}/config.yaml
sed -i -e "s~##DIR##~${DIR}~g" ${DIR}/config.yaml

if [[ "${OS}" == "linux" ]]; then

    # SELinux prevents you from running a system service where the binary is in a user's home directory.
    # We have to copy the binary to a proper directory, such as /usr/local/bin 
    if [[ "${DISTRO}" == "CentOS"* ]] && [[ $(getenforce) != "Disabled" ]] &&  [[ "$(pwd)" == "/home"* ]]; then
        BINDIR=/usr/local/bin
        echo "SELinux enabled, cannot install in home, copy Easegress to ${BINDIR}"
        sudo cp -f ${DIR}/bin/* ${BINDIR}
    fi

    # Prepare the unit file for Systemd
    echo "Configuring the systemd unit file..."
    curl -sL ${RAW_GITHUB_URL}/main/scripts/easegress.service -o ${DIR}/easegress.service
    sed -i -e "s~##BINDIR##~${BINDIR}~g" ${DIR}/easegress.service
    sed -i -e "s~##DIR##~${DIR}~g" ${DIR}/easegress.service
    
    # install the systemd unit file
    echo "Enable the easegress service"
    sudo cp -f ${DIR}/easegress.service /etc/systemd/system 
    sudo systemctl daemon-reload
    sudo systemctl enable easegress
    
    echo "Start the easegress service"
    sudo systemctl start easegress
    
    #check the status
    systemctl -q is-active easegress.service  && \
        echo "Easegress service is running!" || \
        systemctl status easegress.service
fi

echo "Installed successfully"
