FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
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

ENV STORAGE_MAX=100GB
ENV DATA_DIR=/data
ENV IPFS_PATH=/data

RUN apk add --no-cache ca-certificates gcompat && \
  adduser -D -h /app originless

WORKDIR /app

COPY --from=kubo /ipfs /usr/local/bin/ipfs
COPY --from=builder /originless /app/originless
COPY public ./public

RUN mkdir -p /tmp/originless /data && chown -R originless:originless /app /tmp/originless /data

USER originless

EXPOSE 3232 4001/tcp 4001/udp 5001

VOLUME ["/data"]

STOPSIGNAL SIGTERM

CMD ["sh", "-c", "\
  if [ ! -f \"$IPFS_PATH/config\" ]; then ipfs init --profile=lowpower; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs config --json Routing.Type '\"dhtclient\"' && \
  ipfs config --json Swarm.RelayService.Enabled false && \
  ipfs config --json Swarm.RelayClient.Enabled true && \
  ipfs daemon --enable-gc --routing=dhtclient & \
  until ipfs id >/dev/null 2>&1; do \
  echo 'Waiting for IPFS daemon...'; sleep 3; \
  done && \
  exec /app/originless"]