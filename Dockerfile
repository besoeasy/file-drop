# Use Node.js 20 as the base image - provides a stable Node environment
FROM node:20

# Define build argument for architecture (amd64 or arm64)
ARG TARGETARCH

# Update package lists and install curl, then clean up to reduce image size
RUN apt-get update && \
    apt-get install -y curl && \
    rm -rf /var/lib/apt/lists/*

# Install specific version of IPFS (kubo) based on target architecture
# Downloads, extracts, moves binary to PATH, and cleans up in one layer
RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.34.0/kubo_v0.34.0_linux-${TARGETARCH}.tar.gz" | \
    tar -xz -C /tmp && \
    mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
    rm -rf /tmp/kubo

# Set working directory for application code
WORKDIR /app

# Copy package files first for better layer caching
COPY package.json package-lock.json* ./

# Install production dependencies only, using cached layer if unchanged
RUN npm ci --omit=dev

# Copy remaining application files
COPY . .

# Expose ports: 3232 (app), 5001 (IPFS API), 8080 (IPFS gateway)
EXPOSE 3232 5001 8080

# Initialize IPFS, start daemon, connect to swarm, and run app
# Using exec form for proper signal handling
CMD ["sh", "-c", "ipfs init && ipfs daemon & ipfs swarm connect /dnsaddr/dweb.link && sleep 5 && exec node app.js"]