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

try_time=0
pid=""
while [ "$pid" = "" ]
do
    sleep 3
    try_time=$(($try_time+1))
    pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
    if [[ $try_time -ge 2 ]]; then
        echo "Error: failed to start $server"
        exit 2
    fi
done

popd > /dev/null


