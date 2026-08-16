package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestDecodeNpubToHex(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedHex string
		expectErr   bool
	}{
		{
			name:        "Jack Dorsey npub",
			input:       "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
			expectedHex: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			expectErr:   false,
		},
		{
			name:        "Fiatjaf npub",
			input:       "npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
			expectedHex: "82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2",
			expectErr:   false,
		},
		{
			name:        "Raw hex pubkey pass-through",
			input:       "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			expectedHex: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			expectErr:   false,
		},
		{
			name:        "Invalid npub",
			input:       "npub1invalid",
			expectedHex: "",
			expectErr:   true,
		},
		{
			name:        "Empty input",
			input:       "",
			expectedHex: "",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hexVal, err := DecodeNpubToHex(tt.input)
			if (err != nil) != tt.expectErr {
				t.Fatalf("DecodeNpubToHex(%q) error = %v, expectErr = %v", tt.input, err, tt.expectErr)
			}
			if hexVal != tt.expectedHex {
				t.Errorf("DecodeNpubToHex(%q) = %q, want %q", tt.input, hexVal, tt.expectedHex)
			}
		})
	}
}

func TestExtractCIDsFromContent(t *testing.T) {
	content := `Check out this image: ipfs://QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm and also https://dweb.link/ipfs/bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu and bare QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco!`
	cids := ExtractCIDsFromContent(content)
	if len(cids) != 3 {
		t.Fatalf("expected 3 CIDs, got %d: %v", len(cids), cids)
	}

	expected := map[string]bool{
		"QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm":                   true,
		"bafybeicg2oxl5gah64cvk44phwsr33m42x3fvwg6b2kdt6v2iylndr2mqu":     true,
		"QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco":                   true,
	}

	for _, cid := range cids {
		if !expected[cid] {
			t.Errorf("unexpected CID found: %s", cid)
		}
	}
}

// TestMockRelayQuery spins up a mock WebSocket Nostr relay server to test QueryRelay and EOSE handling.
func TestMockRelayQuery(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mockEvents := []NostrEvent{
		{
			ID:        "event1",
			PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			CreatedAt: 1700000000,
			Kind:      1,
			Content:   "Hello Nostr! Check ipfs://QmZtmD2qtMeK4B6usxBaTm2WpuFiHg8wf5YDrQj83b27Bm",
		},
		{
			ID:        "event2",
			PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d",
			CreatedAt: 1700000100,
			Kind:      1,
			Content:   "Second post",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}

		var req []any
		if err := json.Unmarshal(msg, &req); err != nil || len(req) < 2 || req[0] != "REQ" {
			return
		}

		subID := req[1].(string)

		// Send events
		for _, evt := range mockEvents {
			evtMsg, _ := json.Marshal([]any{"EVENT", subID, evt})
			if err := ws.WriteMessage(websocket.TextMessage, evtMsg); err != nil {
				return
			}
		}

		// Send EOSE
		eoseMsg, _ := json.Marshal([]any{"EOSE", subID})
		_ = ws.WriteMessage(websocket.TextMessage, eoseMsg)

		// Read until close
		_, _, _ = ws.ReadMessage()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var collected []NostrEvent
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := QueryRelay(ctx, wsURL, NostrFilter{Authors: []string{"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"}}, func(evt NostrEvent) {
		collected = append(collected, evt)
	})

	if err != nil {
		t.Fatalf("QueryRelay failed: %v", err)
	}

	if len(collected) != 2 {
		t.Fatalf("expected 2 events, got %d", len(collected))
	}
}
