# This docker file require binary files located at directory 'build/bin/',
# and the binary files must be built for Linux x64 platform.
# But the standard 'make build' command generates binary files at 'bin/',
# and may not target for Linux x64.
#
# To build docker images from this file, please build with command:
#    make build_docker
FROM alpine:3.17

WORKDIR /opt/easegress

COPY build/package/entrypoint.sh /
COPY build/bin/egctl build/bin/easegress-server /opt/easegress/bin/

RUN apk --no-cache add tini tzdata && \
        chmod +x /entrypoint.sh /opt/easegress/bin/*

ENV PATH /opt/easegress/bin:$PATH

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
