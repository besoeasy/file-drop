FROM docker.io/node:lts-slim

ENV STORAGE_MAX=200GB

RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.39.0/kubo_v0.39.0_linux-$(dpkg --print-architecture).tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
    rm -rf /tmp/kubo

WORKDIR /app

COPY package.json package-lock.json* ./

RUN npm i

COPY . .

# Create temp uploads directory and set ownership
RUN mkdir -p /tmp/filedrop && chown -R node:node /app /tmp/filedrop

# Switch to non-root user
USER node

EXPOSE 3232 4001/tcp 4001/udp

# Health check - wait 60s for IPFS to connect to peers
HEALTHCHECK --interval=30s --timeout=10s --start-period=7m --retries=5 \
  CMD curl -f http://localhost:3232/health || exit 1

CMD ["sh", "-c", "\
  if [ ! -d \"$HOME/.ipfs\" ]; then ipfs init; fi && \
  ipfs config Datastore.StorageMax ${STORAGE_MAX} && \
  ipfs daemon --enable-gc & \
  until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
    echo 'Waiting for IPFS daemon...'; sleep 3; \
  done && \
  exec node app.js"]
