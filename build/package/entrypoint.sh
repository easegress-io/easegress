#!/bin/sh

# Support the following running mode
# 1) run easegress without any arguments
# 2) run easegress with easegress arguments
# 3) run the command in easegress container

# docker run megaease/easegress
if [ "$#" -eq 0 ]; then
  exec /opt/easegress/bin/easegress-server --api-addr 0.0.0.0:2381
# docker run megaease/easegress -f config.yaml
elif [ "$1" != "--" ] && [ "$(echo $1 | head -c 1)" == "-" ] ; then
  exec /opt/easegress/bin/easegress-server "$@"
# docker run -it --rm megaease/easegress /bin/sh
# docker run -it --rm megaease/easegress /bin/echo hello world
else
  exec "$@"
fi