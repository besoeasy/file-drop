FROM node:lts

# Set working directory for application code
WORKDIR /app

# Copy package files first for better layer caching
COPY package.json package-lock.json* ./

# Install production dependencies and kubo (IPFS)
RUN npm ci && npm install -g kubo

# Set default storage max (can be overridden with environment variable)
ENV STORAGE_MAX=200GB

# Initialize IPFS repo and configure GC + storage limits
RUN npx kubo init && \
    npx kubo config Datastore.GCPeriod 2h && \
    npx kubo bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN && \
    npx kubo bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa && \
    npx kubo bootstrap add /dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zp9VUdgHqVQggUP9WJA9jJ6F7HpLFq && \
    npx kubo bootstrap add /ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ && \
    npx kubo bootstrap add /ip4/104.236.179.241/tcp/4001/p2p/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM

# Copy remaining application files
COPY . .

# Expose necessary ports
EXPOSE 3232 4001/tcp 4001/udp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:5001/api/v0/id || exit 1


# Start IPFS daemon with GC enabled, wait for it, then run the app
CMD ["sh", "-c", "\
    npx kubo config Datastore.StorageMax ${STORAGE_MAX} && \
    npx kubo daemon --enable-gc & \
    until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
    echo 'Waiting for IPFS daemon...'; sleep 5; \
    done && \
    exec node app.js"]   