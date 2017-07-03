FROM alpine:3.6

COPY . /
RUN apk add --no-cache tini libc6-compat && chmod +x /opt/easegateway/bin/*

ENTRYPOINT ["/sbin/tini", "-g", "--"]
CMD ["/opt/easegateway/bin/easegateway-server"]
