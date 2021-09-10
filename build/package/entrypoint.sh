#!/bin/sh
set -e

if [ $# != 0  ] ; then
  exec "$@"
else
  exec /opt/easegress/bin/easegress-server "$@"
fi

