# This docker file require binary files located at directory 'build/bin/',
# and the binary files must be built for Linux x64 platform.
# But the standard 'make build' command generates binary files at 'bin/',
# and may not target for Linux x64.
#
# To build docker images from this file, please build with command:
#    make build_docker
FROM alpine:3

WORKDIR /opt/easegress

COPY build/package/entrypoint.sh /
COPY build/bin/egctl build/bin/easegress-server build/bin/egbuilder /opt/easegress/bin/

# If the following apk command takes too much time or failed because of
# network issues, we can setup a proxy for it.
#
# ENV http_proxy 'http://192.168.1.1:7890'
# ENV https_proxy 'http://192.168.1.1:7890'

RUN apk --no-cache add tini tzdata && \
        chmod +x /entrypoint.sh /opt/easegress/bin/*

# If we setup a proxy for the apk command, we must unset it
# ENV http_proxy=
# ENV https_proxy=

ENV PATH /opt/easegress/bin:$PATH

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
