#!/bin/bash
pushd `dirname $0` > /dev/null
SCRIPTPATH=`pwd -P`
popd > /dev/null
SCRIPTFILE=`basename $0`

if [ -f ${SCRIPTPATH}/deploy.env ];
then
  source ${SCRIPTPATH}/deploy.env
else
  echo "Can't found deploy.env file"
  exit 1
fi

option=$1
if [ "$option" = "-l" -o "$option" = "--long" ]
then
    ${EG1_EGCTL} --server ${EG1_API} member list 
    exit $?
fi

RED='\033[0;31m'
NC='\033[0m' # No Color

for i in {1..10}
do
{
    echo "Cluster Member Role ETCD Status LocalPeer Client API"
{
    ${EG1_EGCTL} --server ${EG1_API} member list  \
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
            timestamp=`date -d "$last" +%s`
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
            if [ $role = "writer" ] 
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
}  | column -t | tee tmp_status.txt 

has_offline=`cat tmp_status.txt|grep offline|wc -l`
    if [ $has_offline = "0" ]
    then
	rm tmp_status.txt
        exit 0
    fi
    sleep 20
done
echo "exceeds 10 times status, has offline node"
rm tmp_status.txt
exit 1
