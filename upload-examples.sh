#!/usr/bin/env bash
set -euo pipefail

# ── Colours ────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ── Helpers ────────────────────────────────────────────────────────
info()  { printf "${CYAN}▸${RESET} %s\n" "$*"; }
ok()    { printf "${GREEN}✔${RESET} %s\n" "$*"; }
warn()  { printf "${YELLOW}⚠${RESET} %s\n" "$*"; }
fail()  { printf "${RED}✖${RESET} %s\n" "$*"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES_DIR="${SCRIPT_DIR}/examples"

# ── Check prerequisites ───────────────────────────────────────────
command -v curl >/dev/null 2>&1 || fail "curl is required but not installed."
command -v jq   >/dev/null 2>&1 || fail "jq is required but not installed."
[[ -d "$EXAMPLES_DIR" ]] || fail "examples/ directory not found at ${EXAMPLES_DIR}"

# ── Prompt for server URL ─────────────────────────────────────────
printf "\n${BOLD}Originless — Upload Examples${RESET}\n\n"
read -rp "Enter the Originless server URL (e.g. http://localhost:8080): " SERVER_URL

# Trim trailing slash
SERVER_URL="${SERVER_URL%/}"

# ── Validate URL format ───────────────────────────────────────────
if [[ ! "$SERVER_URL" =~ ^https?:// ]]; then
  fail "Invalid URL — must start with http:// or https://"
fi

# ── Health check ──────────────────────────────────────────────────
info "Verifying server at ${SERVER_URL} ..."

HEALTH_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${SERVER_URL}/health" 2>/dev/null) || true

if [[ "$HEALTH_RESPONSE" != "200" ]]; then
  fail "Server returned HTTP ${HEALTH_RESPONSE:-"no response"} on /health. Is the server running?"
fi

HEALTH_BODY=$(curl -s --max-time 5 "${SERVER_URL}/health")
HEALTH_STATUS=$(echo "$HEALTH_BODY" | jq -r '.status // empty')

if [[ "$HEALTH_STATUS" != "healthy" ]]; then
  fail "Server reports status '${HEALTH_STATUS:-unknown}'. Please ensure IPFS is connected."
fi

PEERS=$(echo "$HEALTH_BODY" | jq -r '.peers // "unknown"')
ok "Server is healthy (${PEERS} peer(s))"

# ── Build multipart form for all files in examples/ ────────────────
info "Uploading examples/ folder ($(ls -1 "$EXAMPLES_DIR" | wc -l) files) ..."

# Build --form arguments: each file is a "file" field
FORM_ARGS=()
for filepath in "${EXAMPLES_DIR}"/*; do
  [[ -f "$filepath" ]] || continue
  filename="$(basename "$filepath")"
  FORM_ARGS+=(-F "file=@${filepath};filename=${filename}")
done

if [[ ${#FORM_ARGS[@]} -eq 0 ]]; then
  fail "No files found in examples/"
fi

# ── Upload ─────────────────────────────────────────────────────────
UPLOAD_RESPONSE=$(curl -s --max-time 120 \
  "${SERVER_URL}/uploadfolder" \
  "${FORM_ARGS[@]}")

# ── Parse result ───────────────────────────────────────────────────
STATUS=$(echo "$UPLOAD_RESPONSE" | jq -r '.status // empty')

if [[ "$STATUS" != "success" ]]; then
  ERROR_MSG=$(echo "$UPLOAD_RESPONSE" | jq -r '.message // .error // "unknown error"')
  fail "Upload failed: ${ERROR_MSG}"
fi

CID=$(echo "$UPLOAD_RESPONSE" | jq -r '.cid')
FILE_COUNT=$(echo "$UPLOAD_RESPONSE" | jq -r '.files // "?"')
FILE_SIZE=$(echo "$UPLOAD_RESPONSE" | jq -r '.size // "?"')
PINNED=$(echo "$UPLOAD_RESPONSE" | jq -r '.pinned // "?"')

echo ""
ok "Upload successful!"
echo ""
printf "  ${BOLD}CID:   ${RESET}%s\n" "$CID"
printf "  ${BOLD}Files: ${RESET}%s\n" "$FILE_COUNT"
printf "  ${BOLD}Size:  ${RESET}%s bytes\n" "$FILE_SIZE"
printf "  ${BOLD}Pinned:${RESET} %s\n" "$PINNED"
echo ""
printf "  ${BOLD}Link:  ${RESET}${SERVER_URL}/archive/%s\n" "$CID"
echo ""
