#!/bin/bash

APPDIR=`dirname $0`
pushd $APPDIR > /dev/null
APPDIR=`pwd`

server=$APPDIR/bin/easegress-server
cfgfile=$APPDIR/conf/config.yaml

pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ -n "$pid" ] 
then 
    alive="yes"
else
    alive="no"
fi

if [ "$alive" = "no" ]
then 
    exit 0
fi

if [ "$1" = "-f" ]
then
    kill -9 $pid
    exit 0
fi


RED='\033[0;31m'
NC='\033[0m' # No Color

total_etcd=`./status.sh | grep primary | wc -l`
online=`./status.sh | grep primary | grep online | wc -l`

if [ $online -le $((total_etcd / 2 + 1)) ]
then
        if [ "$1" != "-f" ]
        then
            echo -e "total etcd members: ${RED}$total_etcd${NC}, online members: ${RED}$online${NC}, I am alive: ${RED}$alive${NC}"
            echo -e "${RED}Stop this node will cause the etcd unavailable${NC}"
            echo -e "Run ${RED}$0 -f${NC} if you really wanna to proceed"
            exit 1
        fi
fi

pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ ! -z $pid ];then
    kill -9 $pid
fi

popd > /dev/null


