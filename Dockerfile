FROM alpine:3.13

COPY . /
RUN apk add --no-cache tini libc6-compat && chmod +x /entrypoint.server.sh && chmod +x /opt/easegress/bin/*

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.server.sh"]
