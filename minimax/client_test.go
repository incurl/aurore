package minimax

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient builds a client pointed at a test server with the rate
// limiter effectively disabled.
func newTestClient(t *testing.T, srv *httptest.Server, key string) *Client {
	t.Helper()
	return NewClient(key,
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithRateLimiter(NewRateLimiter(1000, time.Millisecond)),
		WithMaxConcurrent(4),
	)
}

func TestGenerateSuccessBase64(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes"))
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			Data:     DataObject{ImageBase64: []string{payload}},
			Metadata: Metadata{SuccessCount: 1},
			ID:       "task-123",
			BaseResp: BaseResp{StatusCode: StatusOK, StatusMsg: "success"},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "secret-key")
	res, err := client.Generate(context.Background(), ImageRequest{Model: ModelImage01, Prompt: "a cat"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-key")
	}
	if len(res.Images) != 1 {
		t.Fatalf("images = %d, want 1", len(res.Images))
	}
	if string(res.Images[0]) != "fake-jpeg-bytes" {
		t.Errorf("image = %q", res.Images[0])
	}
	if res.TaskID != "task-123" {
		t.Errorf("task id = %q, want task-123", res.TaskID)
	}
	if gotBody["prompt"] != "a cat" {
		t.Errorf("sent prompt = %v", gotBody["prompt"])
	}
}

func TestGenerateSuccessURLs(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == ImageGenerationPath {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ImageResponse{
				Data:     DataObject{ImageURLs: []string{srv.URL + "/img.png"}},
				Metadata: Metadata{SuccessCount: 1},
				BaseResp: BaseResp{StatusCode: StatusOK},
			})
			return
		}
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	res, err := client.Generate(context.Background(), ImageRequest{
		Model: ModelImage01, Prompt: "x", ResponseFormat: ResponseFormatURL,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Images) != 1 || string(res.Images[0]) != "png-bytes" {
		t.Errorf("images = %q", res.Images)
	}
}

func TestGenerateMapsErrorCodes(t *testing.T) {
	cases := []struct {
		name  string
		code  int
		check func(*testing.T, *Error)
	}{
		{"rate limited", StatusRateLimited, func(t *testing.T, e *Error) {
			if !e.IsRateLimited() {
				t.Error("expected a rate limit error")
			}
		}},
		{"auth failed", StatusAuthFailed, func(t *testing.T, e *Error) {
			if !e.IsAuthError() {
				t.Error("expected an auth error")
			}
		}},
		{"insufficient balance", StatusInsufficientBalance, func(t *testing.T, e *Error) {
			if !e.IsInsufficientBalance() {
				t.Error("expected an insufficient balance error")
			}
		}},
		{"sensitive content", StatusSensitiveContent, func(t *testing.T, e *Error) {
			if !e.IsContentPolicy() {
				t.Error("expected a content policy error")
			}
		}},
		{"invalid params", StatusInvalidParams, func(t *testing.T, e *Error) {
			if !e.IsInvalidParams() {
				t.Error("expected an invalid params error")
			}
		}},
		{"invalid key", StatusInvalidAPIKey, func(t *testing.T, e *Error) {
			if !e.IsAuthError() {
				t.Error("expected an auth error")
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(ImageResponse{
					BaseResp: BaseResp{StatusCode: tc.code, StatusMsg: tc.name},
				})
			}))
			defer srv.Close()

			client := newTestClient(t, srv, "key")
			_, err := client.Generate(context.Background(), ImageRequest{Model: ModelImage01, Prompt: "x"})
			if err == nil {
				t.Fatal("expected an error")
			}
			apiErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("error type = %T, want *minimax.Error", err)
			}
			if apiErr.StatusCode != tc.code {
				t.Errorf("status = %d, want %d", apiErr.StatusCode, tc.code)
			}
			tc.check(t, apiErr)
			if apiErr.HumanMessage() == "" {
				t.Error("HumanMessage must not be empty")
			}
		})
	}
}

func TestGenerateWithoutAPIKeyFails(t *testing.T) {
	client := NewClient("   ")
	_, err := client.Generate(context.Background(), ImageRequest{Model: ModelImage01, Prompt: "x"})
	if err == nil {
		t.Fatal("expected an error without an API key")
	}
	apiErr, ok := err.(*Error)
	if !ok || !apiErr.IsAuthError() {
		t.Errorf("error = %v, want an auth error", err)
	}
}

func TestGenerateHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	_, err := client.Generate(context.Background(), ImageRequest{Model: ModelImage01, Prompt: "x"})
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %v (%T), want *minimax.Error", err, err)
	}
	if !apiErr.IsRateLimited() {
		t.Errorf("status = %d, want a rate limit error", apiErr.StatusCode)
	}
}

func TestPingAcceptsValidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			BaseResp: BaseResp{StatusCode: StatusInvalidParams, StatusMsg: "prompt is required"},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPingRejectsInvalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ImageResponse{
			BaseResp: BaseResp{StatusCode: StatusAuthFailed, StatusMsg: "invalid api key"},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "bad-key")
	err := client.Ping(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*Error)
	if !ok || !apiErr.IsAuthError() {
		t.Errorf("error = %v, want an auth error", err)
	}
}

func TestPingWithoutKeyFails(t *testing.T) {
	client := NewClient("")
	if err := client.Ping(context.Background()); err == nil {
		t.Fatal("expected an error without an API key")
	}
}

func TestErrorHumanMessageIsUserFacing(t *testing.T) {
	e := &Error{StatusCode: StatusRateLimited, Message: "raw"}
	if !strings.Contains(e.HumanMessage(), "Rate limited") {
		t.Errorf("HumanMessage = %q", e.HumanMessage())
	}
}
