#!/bin/sh
set -eu

export IPFS_PATH="${IPFS_PATH:-/data}"

truthy() {
  v=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
  case "$v" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

falsey() {
  v=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
  case "$v" in
    0|false|no|off) return 0 ;;
    *) return 1 ;;
  esac
}

csv_foreach() {
  # csv_foreach "a,b,c" → prints each trimmed token on its own line
  printf '%s' "$1" | tr ',' '\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sed '/^$/d'
}

json_string_array_from_csv() {
  printf '%s' "$1" | awk -F',' '{
    printf "["
    for (i = 1; i <= NF; i++) {
      gsub(/^[ \t]+|[ \t]+$/, "", $i)
      if ($i == "") next
      if (n++) printf ","
      printf "\"%s\"", $i
    }
    printf "]"
  }'
}

host_from_url() {
  # http://node-a:3232 → node-a   | https://originless.gupt.app/ → originless.gupt.app
  printf '%s' "$1" | sed -E 's#^[a-zA-Z][a-zA-Z0-9+.-]*://##' | sed 's#[/?].*##' | sed 's#:[0-9]*$##'
}

wait_ipfs_api() {
  i=0
  while [ "$i" -lt 90 ]; do
    if ipfs id >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

# Dial other Originless nodes by reading their /status peer id, then swarm-connect.
# IPFS_PEER_NODES=http://node-a:3232,https://originless.example
connect_peer_nodes() {
  [ -n "${IPFS_PEER_NODES:-}" ] || return 0
  csv_foreach "$IPFS_PEER_NODES" | while read -r base; do
    base=$(printf '%s' "$base" | sed 's#/*$##')
    host=$(host_from_url "$base")
    [ -n "$host" ] || continue

    peer=""
    j=0
    while [ "$j" -lt 30 ]; do
      body=$(wget -qO- "$base/status" 2>/dev/null || true)
      peer=$(printf '%s' "$body" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)
      if [ -n "$peer" ]; then
        break
      fi
      j=$((j + 1))
      sleep 2
    done
    [ -n "$peer" ] || continue

    echo "[entrypoint] swarm connect → $host ($peer)"
    ipfs swarm connect "/dns4/${host}/tcp/4001/p2p/${peer}" >/dev/null 2>&1 || true
    ipfs swarm connect "/dns4/${host}/udp/4001/quic-v1/p2p/${peer}" >/dev/null 2>&1 || true
    # Literal IP hosts: also try /ip4 when host looks like an IPv4 address.
    case "$host" in
      *[!0-9.]* ) ;;
      *)
        ipfs swarm connect "/ip4/${host}/tcp/4001/p2p/${peer}" >/dev/null 2>&1 || true
        ipfs swarm connect "/ip4/${host}/udp/4001/quic-v1/p2p/${peer}" >/dev/null 2>&1 || true
        ;;
    esac
  done
}

connect_swarm_multiaddrs() {
  [ -n "${IPFS_SWARM_CONNECT:-}" ] || return 0
  csv_foreach "$IPFS_SWARM_CONNECT" | while read -r addr; do
    echo "[entrypoint] swarm connect → $addr"
    ipfs swarm connect "$addr" >/dev/null 2>&1 || true
  done
}

if [ ! -f "$IPFS_PATH/config" ]; then
  # server profile: suitable for a pin/host node (not the lowpower laptop profile).
  ipfs init --profile=server
fi

ipfs config Datastore.StorageMax "${STORAGE_MAX:-100GB}"

# Re-apply swarm listen on every boot so Docker-published 4001 is actually bound.
ipfs config --json Addresses.Swarm '[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip4/0.0.0.0/udp/4001/quic-v1",
  "/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
  "/ip6/::/tcp/4001",
  "/ip6/::/udp/4001/quic-v1",
  "/ip6/::/udp/4001/quic-v1/webtransport"
]'

# NAT traversal so other Kubo peers can Bitswap pins from this node.
ipfs config --json Swarm.EnableHolePunching true
ipfs config --json Swarm.RelayClient.Enabled true

# Routing mode (dhtclient | dht | auto). dhtclient is the default: announce + query
# without serving as a full DHT server.
routing=$(printf '%s' "${IPFS_ROUTING:-dhtclient}" | tr '[:upper:]' '[:lower:]')
case "$routing" in
  dht|dhtclient|dhtserver|auto|none) ;;
  *) routing=dhtclient ;;
esac
ipfs config --json Routing.Type "\"$routing\""

# Gateway.NoFetch=false (default) lets this node retrieve CIDs from the swarm
# when a client hits /ipfs/{cid} — the Kubo “retrieve and publish” path.
# Set GATEWAY_NO_FETCH=true on public hosts that should only serve local pins.
if truthy "${GATEWAY_NO_FETCH:-false}"; then
  ipfs config --json Gateway.NoFetch true
else
  ipfs config --json Gateway.NoFetch false
fi

# Optional public multiaddrs when Docker publishes 4001 behind NAT/firewall.
# Comma-separated, e.g. /ip4/203.0.113.10/tcp/4001,/ip4/203.0.113.10/udp/4001/quic-v1
if [ -n "${SWARM_ANNOUNCE:-}" ]; then
  announce_json=$(json_string_array_from_csv "$SWARM_ANNOUNCE")
  if [ -n "$announce_json" ] && [ "$announce_json" != "[]" ]; then
    ipfs config --json Addresses.Announce "$announce_json"
  fi
fi

# Sticky Peering.Peers JSON array, e.g.
# [{"ID":"12D3KooW...","Addrs":["/ip4/10.0.0.2/tcp/4001"]}]
if [ -n "${IPFS_PEERING:-}" ]; then
  ipfs config --json Peering.Peers "$IPFS_PEERING"
fi

# Re-applied on every boot so ENABLE_GATEWAY can be toggled without re-init.
if falsey "${ENABLE_GATEWAY:-true}"; then
  ipfs config Addresses.Gateway /ip4/127.0.0.1/tcp/8080
else
  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080
fi

ipfs daemon --enable-gc --routing="$routing" &
export IPFS_PID=$!

wait_ipfs_api || echo "[entrypoint] warning: IPFS API not ready after wait"

connect_swarm_multiaddrs
connect_peer_nodes

exec /app/originless
