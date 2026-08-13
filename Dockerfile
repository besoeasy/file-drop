FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /originless .

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
  attempts=0; \
  until nc -z 127.0.0.1 5001 >/dev/null 2>&1; do \
    attempts=$((attempts+1)); \
    if [ \"$attempts\" -gt 5 ]; then \
      echo 'IPFS daemon failed to start after 5 attempts'; \
      exit 1; \
    fi; \
    rm -f \"$IPFS_PATH/repo.lock\"; \
    pkill -x ipfs 2>/dev/null || true; \
    while pgrep -x ipfs >/dev/null 2>&1; do sleep 1; done; \
    ipfs daemon --enable-gc --routing=dhtclient & \
    sleep 3; \
  done; \
  exec /app/originless"]