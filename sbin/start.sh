#!/bin/bash

APPDIR=`dirname $0`
pushd $APPDIR > /dev/null
APPDIR=`pwd`

server=$APPDIR/bin/easegress-server
stdfile=$APPDIR/log/std.log
cfgfile=$APPDIR/conf/config.yaml

mkdir -p $APPDIR/log

pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ "$pid" != "" ]; then
    echo "$server is running"
fi

touch  $stdfile
nohup $server "$@" --config-file $cfgfile   >> $stdfile 2>&1 &

pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ "$pid" = "" ]; then
    echo "Error: failed to start $sever"
    exit 2
fi

popd > /dev/null


