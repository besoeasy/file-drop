package modules

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// NostrEvent represents a standard Nostr event (NIP-01).
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// NostrFilter represents a subscription filter (NIP-01).
type NostrFilter struct {
	IDs     []string `json:"ids,omitempty"`
	Authors []string `json:"authors,omitempty"`
	Kinds   []int    `json:"kinds,omitempty"`
	Since   int64    `json:"since,omitempty"`
	Until   int64    `json:"until,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

// FetchOptions configures Nostr post fetching.
type FetchOptions struct {
	Relays     []string
	Kinds      []int
	Since      int64
	Until      int64
	Limit      int
	PageSize   int
	MaxPages   int
	ReqTimeout time.Duration
}

// DefaultFetchKinds includes notes, reposts, pictures, NIP-94 files, long-form, and highlights.
var DefaultFetchKinds = []int{1, 6, 20, 1063, 30023, 30024, 9802}

// ArchiveFetchKinds is the subset scanned for IPFS media to pin permanently.
var ArchiveFetchKinds = []int{1, 6, 20, 1063, 30023, 30024, 9802}

func randomSubID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sub_%d", time.Now().UnixNano())
	}
	return "sub_" + hex.EncodeToString(b)
}

// QueryRelay connects to a single Nostr relay, submits a REQ filter, streams events to callback until EOSE/timeout, and closes cleanly.
func QueryRelay(ctx context.Context, relayURL string, filter NostrFilter, onEvent func(NostrEvent)) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}

	conn, _, err := dialer.DialContext(ctx, relayURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", relayURL, err)
	}
	defer conn.Close()

	subID := randomSubID()
	reqPayload, err := json.Marshal([]any{"REQ", subID, filter})
	if err != nil {
		return fmt.Errorf("marshal REQ: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, reqPayload); err != nil {
		return fmt.Errorf("write REQ to %s: %w", relayURL, err)
	}

	// Graceful close helper
	sendClose := func() {
		closeMsg, _ := json.Marshal([]any{"CLOSE", subID})
		_ = conn.WriteMessage(websocket.TextMessage, closeMsg)
	}
	defer sendClose()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var envelope []json.RawMessage
		if err := json.Unmarshal(msg, &envelope); err != nil || len(envelope) < 2 {
			continue
		}

		var verb string
		if err := json.Unmarshal(envelope[0], &verb); err != nil {
			continue
		}

		switch verb {
		case "EVENT":
			if len(envelope) >= 3 {
				var msgSubID string
				if err := json.Unmarshal(envelope[1], &msgSubID); err == nil && msgSubID == subID {
					var event NostrEvent
					if err := json.Unmarshal(envelope[2], &event); err == nil && event.ID != "" {
						onEvent(event)
					}
				}
			}
		case "EOSE":
			if len(envelope) >= 2 {
				var msgSubID string
				if err := json.Unmarshal(envelope[1], &msgSubID); err == nil && msgSubID == subID {
					return nil // End of stored events reached
				}
			}
		case "CLOSED":
			return nil
		}
	}
}

// QueryRelays queries multiple relays in parallel and returns deduplicated events sorted by created_at DESC.
func QueryRelays(ctx context.Context, relays []string, filter NostrFilter, timeout time.Duration) ([]NostrEvent, error) {
	if len(relays) == 0 {
		relays = NostrRelays
	}
	if len(relays) == 0 {
		relays = DefaultFamousRelays
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		mu     sync.Mutex
		events = make(map[string]NostrEvent)
		wg     sync.WaitGroup
	)

	for _, rURL := range relays {
		rURL := rURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = QueryRelay(ctx, rURL, filter, func(evt NostrEvent) {
				mu.Lock()
				events[evt.ID] = evt
				mu.Unlock()
			})
		}()
	}

	wg.Wait()

	result := make([]NostrEvent, 0, len(events))
	for _, evt := range events {
		result = append(result, evt)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})

	return result, nil
}

// FetchAllUserPosts fetches all posts for a single Nostr account (by npub or hex pubkey) across relays with pagination.
func FetchAllUserPosts(ctx context.Context, npubOrHex string, opts FetchOptions) ([]NostrEvent, error) {
	pubkeyHex, err := DecodeNpubToHex(npubOrHex)
	if err != nil {
		return nil, fmt.Errorf("invalid account pubkey/npub: %w", err)
	}

	relays := opts.Relays
	if len(relays) == 0 {
		relays = NostrRelays
	}
	if len(relays) == 0 {
		relays = DefaultFamousRelays
	}

	kinds := opts.Kinds
	if len(kinds) == 0 {
		kinds = DefaultFetchKinds
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 250
	}

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 50
	}

	reqTimeout := opts.ReqTimeout
	if reqTimeout <= 0 {
		reqTimeout = 8 * time.Second
	}

	var (
		allEvents = make(map[string]NostrEvent)
		until     = opts.Until
		pageCount = 0
	)

	if until <= 0 {
		until = time.Now().Unix()
	}

	for pageCount < maxPages {
		filter := NostrFilter{
			Authors: []string{pubkeyHex},
			Kinds:   kinds,
			Since:   opts.Since,
			Until:   until,
			Limit:   pageSize,
		}

		events, err := QueryRelays(ctx, relays, filter, reqTimeout)
		if err != nil || len(events) == 0 {
			break
		}

		newCount := 0
		minTimestamp := until
		for _, evt := range events {
			if _, exists := allEvents[evt.ID]; !exists {
				allEvents[evt.ID] = evt
				newCount++
			}
			if evt.CreatedAt < minTimestamp {
				minTimestamp = evt.CreatedAt
			}
		}

		if newCount == 0 || minTimestamp >= until || (opts.Limit > 0 && len(allEvents) >= opts.Limit) {
			break
		}

		until = minTimestamp - 1
		pageCount++
	}

	result := make([]NostrEvent, 0, len(allEvents))
	for _, evt := range allEvents {
		result = append(result, evt)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})

	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[:opts.Limit]
	}

	return result, nil
}

// FetchPostsForConfiguredAccounts queries posts for all configured NostrNpubs and returns them mapped by account.
func FetchPostsForConfiguredAccounts(ctx context.Context, opts FetchOptions) (map[string][]NostrEvent, error) {
	if len(NostrNpubs) == 0 {
		return map[string][]NostrEvent{}, nil
	}

	results := make(map[string][]NostrEvent)
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for _, npub := range NostrNpubs {
		npub := npub
		wg.Add(1)
		go func() {
			defer wg.Done()
			posts, err := FetchAllUserPosts(ctx, npub, opts)
			if err != nil {
				log.Printf("[nostr] error fetching posts for %s: %v", npub, err)
				return
			}
			mu.Lock()
			results[npub] = posts
			mu.Unlock()
			log.Printf("[nostr] fetched %d posts for %s", len(posts), npub)
		}()
	}

	wg.Wait()
	return results, nil
}
