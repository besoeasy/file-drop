FROM node:20

ARG TARGETARCH
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Directly use TARGETARCH (expects "amd64" or "arm64")
RUN curl -fsSL "https://dist.ipfs.tech/kubo/v0.34.0/kubo_v0.34.0_linux-${TARGETARCH}.tar.gz" | tar -xz && \
    mv kubo/ipfs /usr/local/bin/ipfs && \
    rm -rf kubo

WORKDIR /app
COPY package*.json ./
RUN npm install --only=production
COPY . .
EXPOSE 3232 5001 8080

CMD ipfs init && ipfs daemon & sleep 5 && node app.js
