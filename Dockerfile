FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /originless ./cmd/originless

FROM alpine:latest

ENV STORAGE_MAX=100GB
ENV PIN_EXPIRY_DAYS=30

RUN apk add --no-cache ca-certificates gcompat kubo && \
  adduser -D -h /app originless

WORKDIR /app

COPY --from=builder /originless /app/originless

RUN mkdir -p /tmp/originless /data && chown -R originless:originless /app /tmp/originless /data

USER originless

EXPOSE 3232 4001/tcp 4001/udp 5001

VOLUME ["/data"]

STOPSIGNAL SIGTERM

CMD ["sh", "-c", "\
  export IPFS_PATH=/data && \
  if [ ! -f \"$IPFS_PATH/config\" ]; then ipfs init --profile=lowpower; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs config --json Routing.Type '\"dhtclient\"' && \
  ipfs config --json Swarm.RelayService.Enabled false && \
  ipfs config --json Swarm.RelayClient.Enabled true && \
  sleep 3 && \
  for i in 1 2 3 4 5; do \
    rm -f \"$IPFS_PATH/repo.lock\"; \
    (ipfs daemon --enable-gc --routing=dhtclient &); \
    for j in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
      nc -z 127.0.0.1 5001 >/dev/null 2>&1 && break 2; \
      sleep 2; \
    done; \
    echo \"IPFS daemon not ready (attempt $i), retrying...\"; \
    pkill -x ipfs 2>/dev/null || true; \
    while pgrep -x ipfs >/dev/null 2>&1; do sleep 1; done; \
    sleep 2; \
  done; \
  exec /app/originless"]