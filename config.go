package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	IPFSAPI          = "http://127.0.0.1:5001"
	Port             = 3232
	Host             = "0.0.0.0"
	UploadTempDir    = "/tmp/originless"
	AppVersion       = "0.5.1"
	MaxConcurrentOps = 3
	PinThreshold     = 75
	JanitorInterval  = 60      // minutes
	PasteLimit       = 1 << 20 // 1 MiB max paste size
)

var (
	StorageMax        string
	StorageMaxBytes   int64
	FileLimit         int64
	PinExpiryDays     = 30
	NostrNpubs        []string
	NostrRelays       []string
	ArchiveDir        string
	ArchiveInterval   int
	ArchiveRepinHours int
)

var DefaultFamousRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://relay.nostr.band",
	"wss://relay.primal.net",
	"wss://nostr.mom",
	"wss://purplerelay.com",
	"wss://offchain.pub",
	"wss://eden.nostr.land",
}

var sizePattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$`)

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var bech32CharValues = func() [256]int8 {
	var tab [256]int8
	for i := range tab {
		tab[i] = -1
	}
	for i, c := range bech32Charset {
		tab[c] = int8(i)
	}
	return tab
}()

func init() {
	StorageMax = envOrDefault("STORAGE_MAX", "100GB")
	PinExpiryDays = envOrDefaultInt("PIN_EXPIRY_DAYS", 30)
	NostrNpubs = envOrDefaultSlice([]string{"NOSTR_NPUBS", "NPUBS", "NPUB", "NOSTR_ALLOWED_NPUBS"}, []string{})
	NostrRelays = envOrDefaultSlice([]string{"NOSTR_RELAYS", "RELAYS"}, DefaultFamousRelays)
	ArchiveDir = envOrDefault("ARCHIVE_DIR", "/archive")
	ArchiveInterval = envOrDefaultInt("ARCHIVE_INTERVAL", 15)
	ArchiveRepinHours = envOrDefaultInt("ARCHIVE_REPIN_HOURS", 6)

	storageMaxBytes, err := ParseSize(StorageMax)
	if err != nil {
		panic(fmt.Sprintf("invalid STORAGE_MAX: %v", err))
	}

	StorageMaxBytes = storageMaxBytes
	FileLimit = storageMaxBytes / 100
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func envOrDefaultSlice(keys []string, fallback []string) []string {
	for _, key := range keys {
		if value := os.Getenv(key); strings.TrimSpace(value) != "" {
			return ParseNpubs(value)
		}
	}
	if fallback == nil {
		return []string{}
	}
	return fallback
}

// ParseNpubs parses an array of Nostr npub keys from a string that can be:
// - A JSON array: e.g. ["npub1...", "npub1..."]
// - A comma-separated string: e.g. "npub1..., npub1..."
// - A semicolon, space, or newline-separated string
// Returns a slice of trimmed, non-empty, deduplicated npub strings.
func ParseNpubs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	var items []string
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		var jsonList []string
		if err := json.Unmarshal([]byte(raw), &jsonList); err == nil {
			items = jsonList
		}
	}

	if items == nil {
		tokens := strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})
		items = tokens
	}

	var result []string
	seen := make(map[string]bool)
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'[]`)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	if result == nil {
		return []string{}
	}
	return result
}

func bech32Polymod(values []byte) uint32 {
	chk := uint32(1)
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32HRPExpand(hrp string) []byte {
	ret := make([]byte, 0, len(hrp)*2+1)
	for i := 0; i < len(hrp); i++ {
		ret = append(ret, byte(hrp[i]>>5))
	}
	ret = append(ret, 0)
	for i := 0; i < len(hrp); i++ {
		ret = append(ret, byte(hrp[i]&31))
	}
	return ret
}

func bech32VerifyChecksum(hrp string, data []byte) bool {
	expanded := bech32HRPExpand(hrp)
	values := append(expanded, data...)
	return bech32Polymod(values) == 1
}

// IsValidNpub reports whether s is a valid bech32-encoded Nostr npub key.
func IsValidNpub(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 63 {
		return false
	}
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "npub1") {
		return false
	}
	hrp := "npub"
	dataPart := lower[5:]
	data := make([]byte, len(dataPart))
	for i := 0; i < len(dataPart); i++ {
		c := dataPart[i]
		val := bech32CharValues[c]
		if val < 0 {
			return false
		}
		data[i] = byte(val)
	}
	return bech32VerifyChecksum(hrp, data)
}

func convertBits(data []byte, fromBits, toBits uint8, pad bool) ([]byte, error) {
	acc := uint32(0)
	bits := uint8(0)
	var ret []byte
	maxv := uint32((1 << toBits) - 1)
	maxAcc := uint32((1 << (fromBits + toBits - 1)) - 1)
	for _, value := range data {
		if uint32(value)>>fromBits != 0 {
			return nil, fmt.Errorf("invalid data range")
		}
		acc = ((acc << fromBits) | uint32(value)) & maxAcc
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	return ret, nil
}

// DecodeNpubToHex converts a Nostr npub key (bech32) or 64-char hex pubkey to a 64-character lowercase hex string.
func DecodeNpubToHex(input string) (string, error) {
	input = strings.TrimSpace(input)
	if len(input) == 64 {
		if _, err := hex.DecodeString(input); err == nil {
			return strings.ToLower(input), nil
		}
	}
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "npub1") || len(lower) != 63 {
		return "", fmt.Errorf("invalid npub format: %s", input)
	}
	hrp := "npub"
	dataPart := lower[5:]
	data := make([]byte, len(dataPart))
	for i := 0; i < len(dataPart); i++ {
		c := dataPart[i]
		val := bech32CharValues[c]
		if val < 0 {
			return "", fmt.Errorf("invalid bech32 character: %c", c)
		}
		data[i] = byte(val)
	}
	if !bech32VerifyChecksum(hrp, data) {
		return "", fmt.Errorf("invalid bech32 checksum for npub: %s", input)
	}
	fiveBitData := data[:len(data)-6]
	eightBitData, err := convertBits(fiveBitData, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %w", err)
	}
	if len(eightBitData) != 32 {
		return "", fmt.Errorf("invalid pubkey length: expected 32 bytes, got %d", len(eightBitData))
	}
	return hex.EncodeToString(eightBitData), nil
}

func ParseSize(sizeStr string) (int64, error) {
	match := sizePattern.FindStringSubmatch(sizeStr)
	if match == nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}

	var unit int64
	switch match[2] {
	case "B", "b":
		unit = 1
	case "KB", "Kb", "kb":
		unit = 1024
	case "MB", "Mb", "mb":
		unit = 1024 * 1024
	case "GB", "Gb", "gb":
		unit = 1024 * 1024 * 1024
	case "TB", "Tb", "tb":
		unit = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit: %s", match[2])
	}

	return int64(math.Floor(value * float64(unit))), nil
}

func FormatBytes(bytes int64) string {
	sizes := []string{"Bytes", "KB", "MB", "GB", "TB"}
	if bytes == 0 {
		return "0 Bytes"
	}

	exp := int(math.Floor(math.Log(float64(bytes)) / math.Log(1024)))
	if exp >= len(sizes) {
		exp = len(sizes) - 1
	}

	value := float64(bytes) / math.Pow(1024, float64(exp))
	return fmt.Sprintf("%.2f %s", value, sizes[exp])
}
