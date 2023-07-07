#!/bin/bash

APPDIR=`dirname $0`
pushd $APPDIR > /dev/null
APPDIR=`pwd`

server=$APPDIR/bin/easegress-server
cfgfile=$APPDIR/conf/config.yaml

option=$1
if [ "$option" = "-l" -o "$option" = "--long" ]
then
    ./egctl.sh get member
    exit $?
fi

RED='\033[0;31m'
NC='\033[0m' # No Color


{
    echo "Cluster Member Role Etcd Status LocalPeer Client API"
{
    ./egctl.sh get member -o yaml \
       | egrep 'cluster-name:|\bname\b:|cluster-role:|lastHeartbeatTime:|peer|client|api-addr|\bstate\b:' \
       | while read line
    do
        if `echo $line | grep -q "cluster-name:"`
        then
            cluster=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q '^[ \t]*name:'`
        then
            name=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q "cluster-role:"`
        then
            role=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q "lastHeartbeatTime:"`
        then
            last=`echo $line | awk '{print $2}' | sed -e 's/\+.*$//' -e 's/"//g' `
            timestamp=`date -j -f "%Y-%m-%dT%H:%M:%S" "$last" +%s`
            now=`date  +%s`
            if [ $((now - timestamp))  -le 6 ]
            then
                status="online"
            else
                status="offline"
            fi
        elif `echo $line | grep -q "cluster-peer-url:"`
        then
            peer=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q "cluster-client-url:"`
        then
            client=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q "api-addr:"`
        then
            api=`echo $line | awk '{print $2}'`
        elif `echo $line | grep -q '\bstate\b:'`
        then
            etcd=`echo $line | awk '{print $2}'`
        fi

        if [ -n "$cluster" ] && [ -n "$name" ] && [ -n "$role" ] && [ -n "$status" ] \
            && [ -n "$peer" ] && [ -n "$client" ] && [ -n $api ]
        then
            if [ $role = "primary" ]
            then
                if [ ! -n "$etcd" ]
                then
                    continue
                else
                    if [ $status = "offline" ]
                    then
                        etcd='Down'
                    fi
                fi
            else
                etcd="Subscriber"
            fi

            echo -e "$cluster $name $role $etcd $status $peer $client $api"

            unset cluster
            unset name
            unset role
            unset status
            unset etcd
        fi
    done
} | sort
} | column -t

pid=`ps -eo pid,args | grep "$server" | grep -v grep | awk '{print $1}'`
if [ -n "$pid" ]
then
    alive="yes"
else
    alive="no"
fi

echo -e ""
echo -e ""
echo "--------------------------------------------------------------------------------"
echo -e "I am alive: ${RED}$alive${NC}"


popd > /dev/null


