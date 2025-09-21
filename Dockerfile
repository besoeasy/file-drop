FROM node:lts

# Define build argument for architecture (amd64 or arm64)
ARG TARGETARCH

# Update package lists and install curl, then clean up to reduce image size
RUN apt-get update && \
    apt-get install -y curl tar && \
    rm -rf /var/lib/apt/lists/*

# Install specific version of IPFS (kubo) based on target architecture
RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.37.0/kubo_v0.37.0_linux-${TARGETARCH}.tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
    rm -rf /tmp/kubo

# Initialize IPFS repo and configure GC + storage limits
RUN ipfs init && \
    ipfs config Datastore.StorageMax 200GB && \
    ipfs config Datastore.GCPeriod 200h && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa && \
    ipfs bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp9VUdgHqVQggUP9WJA9jJ6F7HpLFq && \
    ipfs bootstrap add /ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ && \
    ipfs bootstrap add /ip4/104.236.179.241/tcp/4001/p2p/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM

# Set working directory for application code
WORKDIR /app

# Copy package files first for better layer caching
COPY package.json package-lock.json* ./

# Install production dependencies only, using cached layer if unchanged
RUN npm ci

# Copy remaining application files
COPY . .

# Expose necessary ports
EXPOSE 3232 4001/tcp 4001/udp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:5001/api/v0/id || exit 1


# Start IPFS daemon with GC enabled, wait for it, then run the app
CMD ["sh", "-c", "\
    ipfs daemon --enable-gc & \
    until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
    echo 'Waiting for IPFS daemon...'; sleep 5; \
    done && \
    exec node app.js"]   