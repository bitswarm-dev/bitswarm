FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG APP_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w -X bitswarm/server/internal/buildinfo.Version=${APP_VERSION}" -o /out/server ./cmd/server

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 swarm \
    && mkdir -p /data /downloads \
    && chown swarm:swarm /data /downloads
COPY --from=build /out/server /usr/local/bin/swarm
USER swarm
ENV PORT=8080 DB_PATH=/data/swarm.db SWARM_DOWNLOADS=/downloads
VOLUME ["/data", "/downloads"]
EXPOSE 8080 51413
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -q --spider http://127.0.0.1:8080/api/auth/status || exit 1
ENTRYPOINT ["swarm"]
