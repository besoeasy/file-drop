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
RUN npx kubo init && npx kubo config Datastore.GCPeriod 2h

# Copy remaining application files
COPY . .

# Expose necessary ports
EXPOSE 3232 4001/tcp 4001/udp

# Start IPFS daemon with GC enabled, wait for it, then run the app
CMD ["sh", "-c", "\
    npx kubo config Datastore.StorageMax ${STORAGE_MAX} && \
    npx kubo daemon --enable-gc & \
    until curl -s http://127.0.0.1:5001/api/v0/id > /dev/null; do \
    echo 'Waiting for IPFS daemon...'; sleep 5; \
    done && \
    exec node app.js"]   