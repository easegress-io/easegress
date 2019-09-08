#!/bin/bash

APPDIR=`dirname $0`
pushd $APPDIR > /dev/null
APPDIR=`pwd`
popd > /dev/null

egctlcmd=$APPDIR/bin/egctl
cfgfile=$APPDIR/conf/config.yaml

apiportal=`grep api-addr $cfgfile | sed -e 's/api-addr//' -e 's/^[ \t]*://'`

$egctlcmd --server $apiportal "$@"



