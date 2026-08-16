package modules

import (
	"net/url"
	"regexp"
	"strings"
)

// IPFSRef is an IPFS object discovered in a Nostr event.
type IPFSRef struct {
	CID      string
	URL      string
	Mime     string
	SHA256   string
	Filename string
	Fallback string
}

var (
	cidV0Pattern = regexp.MustCompile(`\bQm[1-9A-HJ-NP-Za-km-z]{44}\b`)
	cidV1Pattern = regexp.MustCompile(`\b[bB][a-z2-7A-Z]{58,}\b`)
	// ipfs://CID, ipfs://ipfs/CID, and any http(s) URL whose path contains /ipfs/CID
	ipfsPathPattern = regexp.MustCompile(`(?i)(?:ipfs://(?:ipfs/)?|/ipfs/)(Qm[1-9A-HJ-NP-Za-km-z]{44}|b[a-z2-7]{58,})`)
	// https://<cid>.ipfs.<gateway>/...
	ipfsSubdomainPattern = regexp.MustCompile(`(?i)https?://(Qm[1-9A-HJ-NP-Za-km-z]{44}|b[a-z2-7]{58,})\.ipfs\.`)
)

const cidTrimCutset = `"'<>(),;[]{}* ` + "`"

// ExtractCIDsFromContent scans event text for IPFS CIDs (v0, v1, ipfs://, and gateway URLs).
func ExtractCIDsFromContent(content string) []string {
	refs := extractRefsFromText(content)
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.CID)
	}
	return out
}

// ExtractIPFSRefs finds IPFS references in event content and tags (NIP-94, NIP-92 imeta, gateways).
func ExtractIPFSRefs(evt NostrEvent) []IPFSRef {
	merged := make(map[string]*IPFSRef)

	add := func(ref IPFSRef) {
		if !ValidCID(ref.CID) {
			return
		}
		existing, ok := merged[ref.CID]
		if !ok {
			cp := ref
			merged[ref.CID] = &cp
			return
		}
		if existing.URL == "" {
			existing.URL = ref.URL
		}
		if existing.Mime == "" {
			existing.Mime = ref.Mime
		}
		if existing.SHA256 == "" {
			existing.SHA256 = ref.SHA256
		}
		if existing.Filename == "" {
			existing.Filename = ref.Filename
		}
		if existing.Fallback == "" {
			existing.Fallback = ref.Fallback
		}
	}

	var fileURL, fileMime, fileSHA, fileName string
	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "url":
			fileURL = tag[1]
			for _, r := range extractRefsFromText(tag[1]) {
				r.URL = firstNonEmpty(r.URL, tag[1])
				add(r)
			}
		case "m":
			fileMime = tag[1]
		case "x":
			if isSHA256Hex(tag[1]) {
				fileSHA = strings.ToLower(tag[1])
			}
		case "filename", "name":
			fileName = tag[1]
		case "imeta":
			add(parseImeta(tag[1:]))
		default:
			for _, part := range tag[1:] {
				for _, r := range extractRefsFromText(part) {
					add(r)
				}
			}
		}
	}

	if evt.Kind == 1063 && fileURL != "" {
		for _, r := range extractRefsFromText(fileURL) {
			r.URL = firstNonEmpty(r.URL, fileURL)
			r.Mime = firstNonEmpty(r.Mime, fileMime)
			r.SHA256 = firstNonEmpty(r.SHA256, fileSHA)
			r.Filename = firstNonEmpty(r.Filename, fileName, filenameFromURL(fileURL))
			add(r)
		}
	}

	for _, r := range extractRefsFromText(evt.Content) {
		add(r)
	}

	out := make([]IPFSRef, 0, len(merged))
	for _, r := range merged {
		if r.Mime == "" {
			r.Mime = fileMime
		}
		if r.SHA256 == "" {
			r.SHA256 = fileSHA
		}
		if r.Filename == "" {
			r.Filename = fileName
		}
		out = append(out, *r)
	}
	return out
}

func parseImeta(parts []string) IPFSRef {
	var ref IPFSRef
	for _, part := range parts {
		key, val, ok := strings.Cut(strings.TrimSpace(part), " ")
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "url":
			ref.URL = val
			if cid := cidFromString(val); cid != "" {
				ref.CID = cid
			}
		case "m":
			ref.Mime = val
		case "x":
			if isSHA256Hex(val) {
				ref.SHA256 = strings.ToLower(val)
			}
		case "filename", "name":
			ref.Filename = val
		case "fallback":
			if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
				ref.Fallback = val
			}
		}
	}
	if ref.Filename == "" {
		ref.Filename = filenameFromURL(ref.URL)
	}
	return ref
}

func extractRefsFromText(text string) []IPFSRef {
	if text == "" {
		return nil
	}
	seen := make(map[string]bool)
	var found []IPFSRef

	add := func(cid, rawURL string) {
		cid = strings.Trim(cid, cidTrimCutset)
		if !ValidCID(cid) || seen[cid] {
			return
		}
		seen[cid] = true
		found = append(found, IPFSRef{
			CID:      cid,
			URL:      rawURL,
			Filename: filenameFromURL(rawURL),
		})
	}

	for _, m := range ipfsPathPattern.FindAllStringSubmatch(text, -1) {
		full := m[0]
		cid := m[1]
		rawURL := ""
		if strings.Contains(strings.ToLower(full), "ipfs://") || strings.Contains(full, "://") {
			rawURL = longestURLAround(text, full)
		} else if idx := strings.Index(text, full); idx >= 0 {
			rawURL = longestURLAround(text, full)
		}
		add(cid, rawURL)
	}
	for _, m := range ipfsSubdomainPattern.FindAllStringSubmatch(text, -1) {
		add(m[1], longestURLAround(text, m[0]))
	}
	for _, cid := range cidV0Pattern.FindAllString(text, -1) {
		add(cid, "")
	}
	for _, cid := range cidV1Pattern.FindAllString(text, -1) {
		add(cid, "")
	}
	return found
}

func cidFromString(s string) string {
	refs := extractRefsFromText(s)
	if len(refs) == 0 {
		return ""
	}
	return refs[0].CID
}

// ValidCID reports whether s looks like a CIDv0 or CIDv1.
func ValidCID(s string) bool {
	if len(s) < 46 || len(s) > 120 {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	if strings.HasPrefix(s, "Qm") && len(s) == 46 && cidV0Pattern.MatchString(s) {
		return true
	}
	lower := strings.ToLower(s)
	if len(s) < 50 || len(s) > 120 {
		return false
	}
	return cidV1Pattern.MatchString(s) && (strings.HasPrefix(lower, "baf") || strings.HasPrefix(lower, "bag") || strings.HasPrefix(lower, "b"))
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range strings.ToLower(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func filenameFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if name == "" || ValidCID(name) {
		return ""
	}
	return name
}

func longestURLAround(text, fragment string) string {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(fragment))
	if idx < 0 {
		return fragment
	}
	start := idx
	for start > 0 {
		c := text[start-1]
		if c == ' ' || c == '\n' || c == '\t' || c == '"' || c == '\'' || c == '<' || c == '>' || c == '(' || c == ')' {
			break
		}
		start--
	}
	end := idx + len(fragment)
	for end < len(text) {
		c := text[end]
		if c == ' ' || c == '\n' || c == '\t' || c == '"' || c == '\'' || c == '<' || c == '>' || c == ')' || c == '(' {
			break
		}
		end++
	}
	return strings.TrimRight(text[start:end], ".,;]")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
