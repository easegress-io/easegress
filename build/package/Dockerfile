FROM alpine:3.13

WORKDIR /opt/easegress

COPY build/package/entrypoint.sh /
COPY build/bin/egctl build/bin/easegress-server /opt/easegress/bin/

RUN apk --no-cache add tini tzdata && \
        chmod +x /entrypoint.sh /opt/easegress/bin/*

ENV PATH /opt/easegress/bin:$PATH

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]

CMD ["easegress-server"]
