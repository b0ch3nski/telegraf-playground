# syntax=docker/dockerfile:1.9.0
ARG GOLANG_VERSION
ARG ALPINE_VERSION

# build layer
FROM --platform=${BUILDPLATFORM} golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS build
SHELL ["/bin/ash", "-euo", "pipefail", "-c"]

RUN apk add --update --no-cache git upx; \
    adduser -D -h /tmp/build build
USER build
WORKDIR /tmp/build

ARG TELEGRAF_VERSION
RUN git clone --depth 1 --single-branch --branch ${TELEGRAF_VERSION} https://github.com/influxdata/telegraf.git
WORKDIR /tmp/build/telegraf

COPY --chown=build ./plugins/ plugins/

ARG TARGETARCH TARGETOS TARGETVARIANT TELEGRAF_TAGS
RUN export GOARCH=${TARGETARCH} GOOS=${TARGETOS} CGO_ENABLED=0; \
    [ "${TARGETARCH}" = "arm" ] && export GOARM="${TARGETVARIANT//v}"; \
    go mod tidy -v; \
    go build -v -trimpath -ldflags "-s -w" -tags "${TELEGRAF_TAGS}" -o telegraf ./cmd/telegraf
RUN upx telegraf

# exec layer
FROM --platform=${TARGETPLATFORM} alpine:${ALPINE_VERSION}

RUN apk add --update --no-cache ip6tables

COPY --chmod=400 telegraf.conf /etc/telegraf/
COPY --from=build --chmod=500 /tmp/build/telegraf /usr/local/bin/
RUN telegraf plugins

CMD ["telegraf", "--watch-config", "notify"]
