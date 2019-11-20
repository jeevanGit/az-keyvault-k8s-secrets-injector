FROM golang:1.12 AS builder
WORKDIR /
COPY . /

FROM alpine:3.8
RUN apk add --no-cache --update ca-certificates curl jq
COPY --from=builder ./bin/secret-injector /usr/local/bin/secret-injector
ENTRYPOINT ["/bin/sh"]
