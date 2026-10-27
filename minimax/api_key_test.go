package minimax

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeAPIKey(t *testing.T) {
	const key = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.abc123"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", key, key},
		{"surrounding spaces", "  " + key + "  ", key},
		{"trailing newline", key + "\n", key},
		{"trailing carriage return and newline", key + "\r\n", key},
		{"leading and trailing tabs", "\t" + key + "\t", key},
		{"line wrapped in the middle", "eyJhbGciOiJSUzI1NiIs\nInR5cCI6IkpXVCJ9", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9"},
		{"double quotes", `"` + key + `"`, key},
		{"single quotes", `'` + key + `'`, key},
		{"backticks", "`" + key + "`", key},
		{"square brackets", "[" + key + "]", key},
		{"angle brackets", "<" + key + ">", key},
		{"bearer prefix", "Bearer " + key, key},
		{"bearer prefix lower case", "bearer " + key, key},
		{"bearer prefix upper case", "BEARER " + key, key},
		{"quoted bearer header", `"Bearer ` + key + `"`, key},
		{"bracketed bearer header", "[Bearer " + key + "]", key},
		{"non-breaking space", "\u00a0" + key + "\u00a0", key},
		{"zero width space", "\u200b" + key, key},
		{"byte order mark", "\ufeff" + key, key},
		{"empty", "", ""},
		{"whitespace only", "   \n\t ", ""},
		{"quotes only", `""`, ""},
		{"bearer only", "Bearer ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeAPIKey(tc.in); got != tc.want {
				t.Errorf("SanitizeAPIKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestNewClientSanitizesTheAPIKey verifies that a key saved with stray
// characters still produces a valid Authorization header.
func TestNewClientSanitizesTheAPIKey(t *testing.T) {
	const key = "abc123"

	for _, raw := range []string{" " + key + " ", `"` + key + `"`, "Bearer " + key, key + "\n"} {
		client := NewClient(raw)
		if client.apiKey != key {
			t.Errorf("NewClient(%q).apiKey = %q, want %q", raw, client.apiKey, key)
		}
	}
}

// TestWithBaseURLTrimsWhitespace verifies that a pasted host is usable.
func TestWithBaseURLTrimsWhitespace(t *testing.T) {
	client := NewClient("key", WithBaseURL("  https://example.test/  "))
	if client.baseURL != "https://example.test" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "https://example.test")
	}
}

// TestFingerprintAPIKey verifies that the logged fingerprint describes
// the key without revealing it.
func TestFingerprintAPIKey(t *testing.T) {
	const key = "eyJhbGciOiJSUzI1NiJ9.abc123"

	got := FingerprintAPIKey(key)
	wantPrefix := fmt.Sprintf("len=%d sha256=", len(key))
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("fingerprint = %q, want it to start with %q", got, wantPrefix)
	}
	if strings.Contains(got, key) {
		t.Errorf("fingerprint %q leaks the key", got)
	}

	// The same key gives the same fingerprint, a different one differs.
	if FingerprintAPIKey(key) != got {
		t.Error("the fingerprint is not stable")
	}
	if FingerprintAPIKey(key+"x") == got {
		t.Error("a different key produced the same fingerprint")
	}

	// A pasted key fingerprints as the clean one.
	if FingerprintAPIKey(`  "Bearer `+key+`"  `) != got {
		t.Error("a pasted key did not fingerprint as the clean one")
	}

	// An empty key is called out.
	if FingerprintAPIKey("   ") != "empty" {
		t.Errorf("FingerprintAPIKey(\"   \") = %q, want %q", FingerprintAPIKey("   "), "empty")
	}

	// An all-digit value is flagged: it cannot be a secret key.
	digits := FingerprintAPIKey("123456789012345678")
	if !strings.Contains(digits, "GroupId") {
		t.Errorf("fingerprint of an all-digit value = %q, want a GroupId warning", digits)
	}
}

func TestGenerateSendsACleanAuthorizationHeader(t *testing.T) {
	const key = "eyJhbGciOiJSUzI1NiJ9.abc123"
	want := "Bearer " + key

	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			ID:       "task-1",
			Data:     DataObject{ImageBase64: []string{base64.StdEncoding.EncodeToString([]byte("fake"))}},
			Metadata: Metadata{SuccessCount: 1},
			BaseResp: BaseResp{StatusCode: StatusOK, StatusMsg: "success"},
		})
	}))
	defer srv.Close()

	pastes := []struct {
		name string
		raw  string
	}{
		{"surrounding spaces", "  " + key + "  "},
		{"double quotes", `"` + key + `"`},
		{"whole header value", "Bearer " + key},
		{"trailing newline", key + "\n"},
		{"non-breaking space", "\u00a0" + key},
		{"bracketed", "[" + key + "]"},
	}

	for _, paste := range pastes {
		t.Run(paste.name, func(t *testing.T) {
			got = ""
			client := newTestClient(t, srv, paste.raw)
			if _, err := client.Generate(context.Background(), ImageRequest{
				Model:  ModelImage01,
				Prompt: "x",
			}); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != want {
				t.Errorf("Authorization = %q, want %q", got, want)
			}
		})
	}
}
