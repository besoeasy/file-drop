#!/usr/bin/env bash
set -euo pipefail

# upload.sh — Upload files/folders/text to an Originless node
# Usage:
#   upload.sh <server-url> file <path>
#   upload.sh <server-url> folder <dir>
#   upload.sh <server-url> paste <text> [title]

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'
info() { printf "${CYAN}▸${RESET} %s\n" "$*"; }
ok()   { printf "${GREEN}✔${RESET} %s\n" "$*"; }
fail() { printf "${RED}✖${RESET} %s\n" "$*"; exit 1; }

usage() {
  cat <<'EOF'
Usage:
  upload.sh <server-url> file <path>
  upload.sh <server-url> folder <dir>
  upload.sh <server-url> paste <text> [title]
EOF
  exit 1
}

[[ $# -ge 2 ]] || usage
SERVER_URL="${1%/}"; shift
command -v curl >/dev/null || fail "curl is required"
command -v jq   >/dev/null || fail "jq is required"
[[ "$SERVER_URL" =~ ^https?:// ]] || fail "Invalid URL — must start with http:// or https://"

# Health check
info "Checking ${SERVER_URL}/health ..."
HEALTH=$(curl -s --max-time 5 "${SERVER_URL}/health" || true)
STATUS=$(echo "$HEALTH" | jq -r '.status // empty')
[[ "$STATUS" == "healthy" ]] || fail "Server not healthy (status: ${STATUS:-no response})"

MODE="$1"; shift

case "$MODE" in
  file)
    [[ $# -eq 1 ]] || usage
    FILE="$1"
    [[ -f "$FILE" ]] || fail "File not found: $FILE"
    info "Uploading file: $FILE"
    RESP=$(curl -s --max-time 120 -X POST -F "file=@${FILE}" "${SERVER_URL}/upload")
    ;;
  folder)
    [[ $# -eq 1 ]] || usage
    DIR="$1"
    [[ -d "$DIR" ]] || fail "Directory not found: $DIR"
    FORM_ARGS=()
    while IFS= read -r -d '' f; do
      rel="${f#"$DIR"/}"
      FORM_ARGS+=(-F "file=@${f};filename=${rel}")
    done < <(find "$DIR" -type f -print0)
    [[ ${#FORM_ARGS[@]} -gt 0 ]] || fail "No files found in $DIR"
    info "Uploading folder: $DIR (${#FORM_ARGS[@]} files)"
    RESP=$(curl -s --max-time 120 -X POST "${SERVER_URL}/uploadfolder" "${FORM_ARGS[@]}")
    ;;
  paste)
    [[ $# -ge 1 ]] || usage
    TEXT="$1"; TITLE="${2:-}"
    info "Pasting text (title: ${TITLE:-none})"
    BODY=$(jq -n --arg content "$TEXT" --arg title "$TITLE" '{content: $content, title: $title}')
    RESP=$(curl -s --max-time 120 -X POST -H "Content-Type: application/json" -d "$BODY" "${SERVER_URL}/paste")
    ;;
  *)
    usage ;;
esac

# Parse response
S=$(echo "$RESP" | jq -r '.status // empty')
if [[ "$S" != "success" ]]; then
  MSG=$(echo "$RESP" | jq -r '.message // .error // "unknown error"')
  fail "Upload failed: $MSG"
fi

CID=$(echo "$RESP" | jq -r '.cid')
echo ""
ok "Upload successful!"
echo "  CID:   $CID"
echo "  IPFS:  ipfs://$CID"
echo "  Link:  https://ipfs.io/ipfs/$CID"
echo "  Alt:   https://dweb.link/ipfs/$CID"