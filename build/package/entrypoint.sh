#!/bin/sh

if [ "$(echo $1 | head -c 1)" != "-" ] ; then
  exec "$@"
else
  exec /opt/easegress/bin/easegress-server "$@"
fi

