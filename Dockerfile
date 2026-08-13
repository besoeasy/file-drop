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

EXPOSE 3232 4001/tcp 4001/udp

VOLUME ["/data"]

STOPSIGNAL SIGTERM

CMD ["sh", "-c", "\
  export IPFS_PATH=/data && \
  if [ ! -f \"$IPFS_PATH/config\" ]; then ipfs init --profile=lowpower; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs config --json Routing.Type '\"dhtclient\"'; \
  ipfs daemon --enable-gc --routing=dhtclient & \
  IPFS_PID=$! && \
  export IPFS_PID && \
  sleep 10 && \
  exec /app/originless"]