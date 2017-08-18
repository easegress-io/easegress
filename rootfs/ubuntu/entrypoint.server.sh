#!/bin/sh

if [ $# -eq 0 ] ; then
  exit 1
fi

if [ "$(echo $1 | head -c 1)" != "-" ] ; then
  exec "$@"
else
  exec /opt/easegateway/bin/easegateway-server "$@"
fi

