#!/usr/bin/env bash

set -e

echo "=================================================="
echo "      Originless IPFS Network Propagation Test     "
echo "=================================================="

# 1. Get Server URL
if [ -n "$1" ]; then
    SERVER_URL="$1"
else
    read -rp "Enter Originless server URL [http://localhost:3232]: " INPUT_URL
    SERVER_URL="${INPUT_URL:-http://localhost:3232}"
fi

# Remove trailing slash if present
SERVER_URL="${SERVER_URL%/}"

# Validate server health
echo ""
echo "🔍 Checking health of server at ${SERVER_URL}..."
if ! curl -s -f --max-time 5 "${SERVER_URL}/health" >/dev/null 2>&1; then
    echo "⚠️ Warning: Could not reach ${SERVER_URL}/health (proceeding anyway...)"
else
    echo "✅ Server is online!"
fi

# 2. Select Gateway for Verification
GATEWAYS=(
    "https://dweb.link"
    "https://ipfs.io"
    "https://cloudflare-ipfs.com"
)

echo ""
echo "Select public IPFS Gateway to test replication against:"
for i in "${!GATEWAYS[@]}"; do
    echo "  $((i+1))) ${GATEWAYS[$i]}"
done
read -rp "Choice [1]: " GW_CHOICE
GW_CHOICE="${GW_CHOICE:-1}"

if [[ "$GW_CHOICE" -ge 1 && "$GW_CHOICE" -le "${#GATEWAYS[@]}" ]]; then
    TEST_GATEWAY="${GATEWAYS[$((GW_CHOICE-1))]}"
else
    TEST_GATEWAY="${GATEWAYS[0]}"
fi
echo "🌐 Selected Gateway: ${TEST_GATEWAY}"

# 3. Generate random file (10KB to 100KB)
RAND_KB=$(( (RANDOM % 91) + 10 ))
SIZE_BYTES=$(( RAND_KB * 1024 ))
TEST_FILE=$(mktemp /tmp/originless_test_XXXXXX.bin)
FILENAME="$(basename "$TEST_FILE").bin"

echo ""
echo "🎲 Generating random payload of ${RAND_KB} KB (${SIZE_BYTES} bytes)..."
dd if=/dev/urandom of="$TEST_FILE" bs=1024 count="$RAND_KB" status=none

# Cleanup temp file on exit
cleanup() {
    rm -f "$TEST_FILE"
}
trap cleanup EXIT

# 4. Upload file to Originless server
echo "🚀 Uploading file to ${SERVER_URL}/upload ..."
UPLOAD_START=$(date +%s)

RESPONSE=$(curl -s -X POST -F "file=@${TEST_FILE};filename=${FILENAME}" "${SERVER_URL}/upload")

UPLOAD_END=$(date +%s)
UPLOAD_DURATION=$(( UPLOAD_END - UPLOAD_START ))

# Extract CID from response
if command -v jq >/dev/null 2>&1; then
    CID=$(echo "$RESPONSE" | jq -r '.cid // empty')
else
    CID=$(echo "$RESPONSE" | grep -o '"cid":"[^"]*"' | cut -d'"' -f4)
fi

if [ -z "$CID" ] || [ "$CID" = "null" ]; then
    echo "❌ Upload failed. Server response:"
    echo "$RESPONSE"
    exit 1
fi

echo "✅ Upload Successful!"
echo "   📄 File Name: ${FILENAME}"
echo "   📦 CID:       ${CID}"
echo "   ⏱️ Upload Time: ${UPLOAD_DURATION} seconds"
echo ""
echo "=================================================="
echo "⏳ Polling gateway (${TEST_GATEWAY}/ipfs/${CID}) every 5s..."
echo "=================================================="

# 5. Poll gateway every 5 seconds until replicated
POLL_START=$(date +%s)
POLL_COUNT=0

while true; do
    POLL_COUNT=$(( POLL_COUNT + 1 ))
    ELAPSED=$(( $(date +%s) - POLL_START ))
    
    # Attempt to fetch CID from public gateway
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" --max-time 4 "${TEST_GATEWAY}/ipfs/${CID}" || true)
    
    if [ "$HTTP_STATUS" -eq 200 ]; then
        TOTAL_TIME=$(( $(date +%s) - UPLOAD_START ))
        REPLICATION_TIME=$(( $(date +%s) - POLL_START ))
        echo ""
        echo "🎉 DATA REPLICATED & AVAILABLE ON NETWORK!"
        echo "--------------------------------------------------"
        echo "   📦 CID:              ${CID}"
        echo "   📏 Size:             ${RAND_KB} KB"
        echo "   ⏱️ Upload Time:      ${UPLOAD_DURATION}s"
        echo "   🌐 Replication Time: ${REPLICATION_TIME}s (Attempt #${POLL_COUNT})"
        echo "   ⏱️ Total Elapsed:    ${TOTAL_TIME}s"
        echo "   🔗 Direct Link:      ${TEST_GATEWAY}/ipfs/${CID}"
        echo "--------------------------------------------------"
        break
    else
        echo "[+${ELAPSED}s] HTTP Status: ${HTTP_STATUS} (Not yet available on gateway). Polling again in 5s..."
        sleep 5
    fi
done
