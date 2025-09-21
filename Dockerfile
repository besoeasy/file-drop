# Multi-stage build for optimal image size
FROM node:lts-alpine AS base

# Build stage - includes build tools
FROM base AS builder
ARG TARGETARCH

# Install curl and other build dependencies
RUN apk add --no-cache curl tar

# Install IPFS
RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.37.0/kubo_v0.37.0_linux-${TARGETARCH}.tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
    rm -rf /tmp/kubo

WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

# Runtime stage - minimal dependencies
FROM base AS runtime
ARG TARGETARCH

# Copy IPFS binary from builder
COPY --from=builder /usr/local/bin/ipfs /usr/local/bin/ipfs

WORKDIR /app

# Copy dependencies and application code
COPY --from=builder /app/node_modules ./node_modules
COPY . .

# Initialize IPFS with optimized configuration
RUN ipfs init && \
    ipfs config Datastore.StorageMax 200GB && \
    ipfs config Datastore.GCPeriod 200h && \
    ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080 && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJyrVwtbZg5gBMjTezGAJN && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp9VUdgHqVQggUP9WJA9jJ6F7HpLFq && \
    ipfs bootstrap add /ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ && \
    ipfs bootstrap add /ip4/104.236.179.241/tcp/4001/p2p/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM

# Expose necessary ports
EXPOSE 3232 4001/tcp 4001/udp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:5001/api/v0/id || exit 1

# Optimized startup script
CMD ["sh", "-c", "\
    ipfs daemon --enable-gc --routing=dhtclient & \
    IPFS_PID=$! && \
    echo 'Starting IPFS daemon...' && \
    until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null 2>&1; do \
        echo 'Waiting for IPFS daemon...'; \
        sleep 2; \
    done && \
    echo 'IPFS daemon ready, starting application...' && \
    exec node app.js"]