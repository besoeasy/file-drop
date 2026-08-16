package main

import (
	"os"
	"reflect"
	"testing"
)

func TestParseNpubs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: []string{},
		},
		{
			name:     "single npub",
			input:    "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
			expected: []string{"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"},
		},
		{
			name:  "comma separated list",
			input: "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6, npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
			expected: []string{
				"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
				"npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
			},
		},
		{
			name:  "json array format",
			input: `["npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6", "npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m"]`,
			expected: []string{
				"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
				"npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
			},
		},
		{
			name:  "semicolon, space, and newline separated",
			input: "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6;\nnpub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m npub1dergggklka99wwrs92yz8wdjs952h2mt20r8x2jj5ww3hw5ecc7s9nnfpn",
			expected: []string{
				"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
				"npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
				"npub1dergggklka99wwrs92yz8wdjs952h2mt20r8x2jj5ww3hw5ecc7s9nnfpn",
			},
		},
		{
			name:     "duplicates removed",
			input:    "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6, npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
			expected: []string{"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"},
		},
		{
			name:  "quoted items stripped",
			input: `"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6", 'npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m'`,
			expected: []string{
				"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
				"npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ParseNpubs(tt.input)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("ParseNpubs(%q) = %v, want %v", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestIsValidNpub(t *testing.T) {
	validKeys := []string{
		"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6",
		"npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m",
		"npub1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzqujme",
		"npub1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqs8j9gdm",
		"npub1qqqsyqcyq5rqwzqfpg9scrgwpugpzysnzs23v9ccrydpk8qarc0st5hsmq",
	}

	for _, k := range validKeys {
		if !IsValidNpub(k) {
			t.Errorf("IsValidNpub(%q) = false, want true", k)
		}
	}

	invalidKeys := []string{
		"",
		"npub1",
		"invalid",
		"nsec1vl029x90zsvuth9rh9rwhntsv6l2atnv2fvjc34qq2860mm80rhq7sqxuz", // nsec, not npub
		"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w7", // corrupted checksum char
		"npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6extra",
	}

	for _, k := range invalidKeys {
		if IsValidNpub(k) {
			t.Errorf("IsValidNpub(%q) = true, want false", k)
		}
	}
}

func TestEnvOrDefaultSlice(t *testing.T) {
	os.Unsetenv("TEST_NOSTR_NPUBS")
	os.Unsetenv("TEST_NPUBS_FALLBACK")

	// Test fallback when unset
	res := envOrDefaultSlice([]string{"TEST_NOSTR_NPUBS", "TEST_NPUBS_FALLBACK"}, []string{"default1"})
	if !reflect.DeepEqual(res, []string{"default1"}) {
		t.Errorf("envOrDefaultSlice fallback = %v, want [default1]", res)
	}

	// Test primary env var
	os.Setenv("TEST_NOSTR_NPUBS", "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6, npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m")
	defer os.Unsetenv("TEST_NOSTR_NPUBS")

	res = envOrDefaultSlice([]string{"TEST_NOSTR_NPUBS", "TEST_NPUBS_FALLBACK"}, []string{})
	if len(res) != 2 {
		t.Errorf("expected 2 keys, got %d", len(res))
	}
}
