#!/bin/sh
set -eu

export IPFS_PATH="${IPFS_PATH:-/data}"

if [ ! -f "$IPFS_PATH/config" ]; then
  ipfs init --profile=lowpower
fi

ipfs config Datastore.StorageMax "${STORAGE_MAX:-100GB}"
ipfs config --json Routing.Type '"dhtclient"'

# Serve only blocks already on this node. The public gateway is not an open
# fetch proxy for arbitrary CIDs from the swarm.
ipfs config --json Gateway.NoFetch true

# Re-applied on every boot so ENABLE_GATEWAY can be toggled without re-init.
enabled=$(printf '%s' "${ENABLE_GATEWAY:-true}" | tr '[:upper:]' '[:lower:]')
case "$enabled" in
  0|false|no|off)
    ipfs config Addresses.Gateway /ip4/127.0.0.1/tcp/8080
    ;;
  *)
    ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080
    ;;
esac

ipfs daemon --enable-gc --routing=dhtclient &
export IPFS_PID=$!
sleep 10
exec /app/originless
