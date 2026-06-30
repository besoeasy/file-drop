FROM golang:1.24-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /originless .

FROM debian:bookworm-slim

ENV STORAGE_MAX=200GB
ENV FILE_LIMIT=5GB
ENV IPFS_PATH=/data

RUN apt-get update && \
  apt-get install -y --no-install-recommends curl ca-certificates tar && \
  rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.42.0/kubo_v0.42.0_linux-$(dpkg --print-architecture).tar.gz" | \
  tar -xz -C /tmp && \
  mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
  rm -rf /tmp/kubo

RUN useradd --system --home /app --create-home originless

WORKDIR /app

COPY --from=builder /originless /app/originless
COPY public ./public

RUN mkdir -p /tmp/originless /data && chown -R originless:originless /app /tmp/originless /data

USER originless

EXPOSE 3232 4001/tcp 4001/udp

VOLUME ["/data"]

STOPSIGNAL SIGTERM
LABEL com.docker.compose.stop-grace-period="15s"

HEALTHCHECK --interval=30s --timeout=10s --start-period=7m --retries=5 \
  CMD curl -f http://localhost:3232/health || exit 1

CMD ["sh", "-c", "\
  if [ ! -f \"$IPFS_PATH/config\" ]; then ipfs init --profile=lowpower; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs config --json Routing.Type '\"dhtclient\"' && \
  ipfs config --json Provide.DHT.Interval '\"24h\"' && \
  ipfs daemon --enable-gc --routing=dhtclient & \
  until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
  echo 'Waiting for IPFS daemon...'; sleep 3; \
  done && \
  exec /app/originless"]