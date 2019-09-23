#!/bin/bash

co_host=(
"ubuntu@192.168.50.101"
"ubuntu@192.168.50.102"
"ubuntu@192.168.50.103"
"ubuntu@192.168.50.104"
"ubuntu@192.168.50.105"
)
co_command=(
"PID=\`cat ~/easegateway/easegateway.pid\`;kill -USR2 \$PID"
)
minintv=60
randintv=600

function gethost(){
	local result=$1
	key=`expr $RANDOM % ${#co_host[@]}`
	eval $result="${co_host[$key]}"
}
function getcommand(){
	local result=$1
	key=`expr $RANDOM % ${#co_command[@]}`
	eval $result='${co_command[$key]}'
}
while true ;
do
	gethost host
	getcommand cmd
	now=`date "+%Y%m%d %H:%M:%S"`
	echo "$host: $cmd ($now)"
	ssh $host $cmd
	intv=`expr $minintv + $RANDOM % $randintv`
	echo "next command will execute after $intv seconds"
	sleep $intv
done
