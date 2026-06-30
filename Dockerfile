FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /originless .

FROM alpine:3.20 AS kubo

RUN apk add --no-cache curl tar && \
  ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && \
  curl -fsSL "https://dist.ipfs.tech/kubo/v0.42.0/kubo_v0.42.0_linux-${ARCH}.tar.gz" | \
  tar -xz -C /tmp && \
  mv /tmp/kubo/ipfs /ipfs && \
  rm -rf /tmp/kubo

FROM alpine:3.20

ENV STORAGE_MAX=200GB
ENV FILE_LIMIT=5GB
ENV IPFS_PATH=/data

RUN apk add --no-cache ca-certificates gcompat && \
  adduser -D -h /app originless

WORKDIR /app

COPY --from=kubo /ipfs /usr/local/bin/ipfs
COPY --from=builder /originless /app/originless
COPY public ./public

RUN mkdir -p /tmp/originless /data && chown -R originless:originless /app /tmp/originless /data

USER originless

EXPOSE 3232 4001/tcp 4001/udp

VOLUME ["/data"]

STOPSIGNAL SIGTERM
LABEL com.docker.compose.stop-grace-period="15s"

HEALTHCHECK --interval=30s --timeout=10s --start-period=7m --retries=5 \
  CMD ["/app/originless", "-health"]

CMD ["sh", "-c", "\
  if [ ! -f \"$IPFS_PATH/config\" ]; then ipfs init --profile=lowpower; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs config --json Routing.Type '\"dhtclient\"' && \
  ipfs config --json Provide.DHT.Interval '\"24h\"' && \
  ipfs daemon --enable-gc --routing=dhtclient & \
  until ipfs id >/dev/null 2>&1; do \
  echo 'Waiting for IPFS daemon...'; sleep 3; \
  done && \
  exec /app/originless"]