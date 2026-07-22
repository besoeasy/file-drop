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
HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${SERVER_URL}/health" || echo "000")
if [ "$HEALTH_STATUS" -ne 200 ]; then
    echo "⚠️ Warning: ${SERVER_URL}/health returned HTTP ${HEALTH_STATUS} (Server may be offline or returning 502 Bad Gateway)"
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

# Capture HTTP status code and body safely
HTTP_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST -F "file=@${TEST_FILE};filename=${FILENAME}" "${SERVER_URL}/upload" || echo -e "\n000")

HTTP_STATUS=$(echo "$HTTP_RESPONSE" | tail -n 1)
RESPONSE=$(echo "$HTTP_RESPONSE" | sed '$d')

UPLOAD_END=$(date +%s)
UPLOAD_DURATION=$(( UPLOAD_END - UPLOAD_START ))

if [ "$HTTP_STATUS" -ne 200 ]; then
    echo ""
    echo "❌ Upload failed with HTTP status: ${HTTP_STATUS}"
    echo "   Server output: ${RESPONSE}"
    if [ "$HTTP_STATUS" -eq 502 ]; then
        echo "   👉 Note: HTTP 502 Bad Gateway means your backend server or Docker container crashed/stopped behind Cloudflare."
    fi
    exit 1
fi

# Extract CID from response
if command -v jq >/dev/null 2>&1; then
    CID=$(echo "$RESPONSE" | jq -r '.cid // empty' 2>/dev/null || true)
else
    CID=$(echo "$RESPONSE" | grep -o '"cid":"[^"]*"' | cut -d'"' -f4)
fi

if [ -z "$CID" ] || [ "$CID" = "null" ]; then
    echo "❌ Could not parse CID from server response:"
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
    
    # Use -L to follow sub-domain redirects (e.g. dweb.link redirects to cid.ipfs.dweb.link)
    HTTP_STATUS=$(curl -s -L -o /dev/null -w "%{http_code}" --max-time 8 "${TEST_GATEWAY}/ipfs/${CID}" || echo "000")
    
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
        echo "[+${ELAPSED}s] Gateway Response: HTTP ${HTTP_STATUS} (Not ready yet). Retrying in 5s..."
        
        if [ "$ELAPSED" -gt 60 ] && [ "$((ELAPSED % 30))" -lt 5 ]; then
            echo "   ⚠️ Note: If this takes longer than 60s, verify that port 4001 (TCP/UDP) is open on your server."
        fi
        
        sleep 5
    fi
done
