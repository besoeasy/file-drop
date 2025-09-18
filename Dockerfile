# Use Node.js 20 as the base image - provides a stable Node environment
FROM node:slim

# Define build argument for architecture (amd64 or arm64)
ARG TARGETARCH

# Update package lists and install curl, then clean up to reduce image size
RUN apt-get update && \
  apt-get install -y curl && \
  rm -rf /var/lib/apt/lists/*

# Install specific version of IPFS (kubo) based on target architecture
RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.37.1/kubo_v0.37.0_linux-${TARGETARCH}.tar.gz" | \
  tar -xz -C /tmp && \
  mv /tmp/kubo/ipfs /usr/local/bin/ipfs && \
  rm -rf /tmp/kubo

# Initialize IPFS repo and configure GC + storage limits
RUN ipfs init && \
  ipfs config Datastore.StorageMax 200GB && \
  ipfs config Datastore.GCPeriod 200h

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

# Start IPFS daemon with GC enabled, wait for it, then run the app
CMD ["sh", "-c", "\
  ipfs daemon --enable-gc & \
  until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
  echo 'Waiting for IPFS daemon...'; sleep 5; \
  done && \
  exec node app.js"]
