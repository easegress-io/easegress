FROM golang:1.16-alpine AS builder
RUN apk --no-cache add make git

WORKDIR /opt/easegress
COPY . .

RUN make clean && make build

# ---

FROM alpine:3.13

WORKDIR /opt/easegress

COPY build/package/entrypoint.sh /
COPY --from=builder /opt/easegress/bin/ /opt/easegress/bin/

RUN apk --no-cache add tini tzdata && \
        chmod +x /entrypoint.sh /opt/easegress/bin/*

ENV PATH /opt/easegress/bin:$PATH

ENTRYPOINT ["/sbin/tini", "--" ,"/entrypoint.sh"]

CMD ["easegress-server"]
