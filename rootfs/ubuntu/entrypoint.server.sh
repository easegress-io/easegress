#!/bin/sh

if [ $# -eq 0 ] ; then
  set -- "-plugin_python_root_namespace" "-plugin_shell_root_namespace"
fi

if [ "$(echo $1 | head -c 1)" != "-" ] ; then
  exec "$@"
else
  exec /opt/easegateway/bin/easegateway-server "$@"
fi

