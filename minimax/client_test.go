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
	"unicode/utf8"
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

// TestGenerateAcceptsTheDocumentedResponse verifies that the response
// MiniMax documents is decoded.
//
// The published example returns the image counts as JSON strings, so a
// strict int field failed the whole decode and the generated images were
// thrown away. This test pins the documented body down.
func TestGenerateAcceptsTheDocumentedResponse(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes"))

	// The example body from the MiniMax API reference, with the image
	// URLs replaced by the base64 payload the request asked for.
	body := `{
		"id": "03ff3cd0820949eb8a410056b5f21d38",
		"data": {"image_base64": ["` + payload + `"]},
		"metadata": {"failed_count": "0", "success_count": "1"},
		"base_resp": {"status_code": 0, "status_msg": "success"}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "secret-key")
	res, err := client.Generate(context.Background(), ImageRequest{
		Model: ModelImage01, Prompt: "a cat",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.TaskID != "03ff3cd0820949eb8a410056b5f21d38" {
		t.Errorf("task id = %q", res.TaskID)
	}
	if len(res.Images) != 1 {
		t.Fatalf("images = %d, want 1", len(res.Images))
	}
	if string(res.Images[0]) != "fake-jpeg-bytes" {
		t.Errorf("image = %q", res.Images[0])
	}
	if res.SuccessCount != 1 {
		t.Errorf("success count = %d, want 1", res.SuccessCount)
	}
	if res.FailedCount != 0 {
		t.Errorf("failed count = %d, want 0", res.FailedCount)
	}
}

// TestGenerateAcceptsStringCounts verifies that the counts are read from
// the string form the API sends.
func TestGenerateAcceptsStringCounts(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte("fake-jpeg-bytes"))
	body := `{"data":{"image_base64":["` + payload + `"]},` +
		`"metadata":{"failed_count":"2","success_count":"5"},` +
		`"base_resp":{"status_code":0,"status_msg":"success"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	res, err := client.Generate(context.Background(), ImageRequest{Model: ModelImage01, Prompt: "x"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.SuccessCount != 5 {
		t.Errorf("success count = %d, want 5", res.SuccessCount)
	}
	if res.FailedCount != 2 {
		t.Errorf("failed count = %d, want 2", res.FailedCount)
	}
}

// TestGenerateAcceptsStringStatusCode verifies that an error reported
// with a stringified status code is still recognised as that error.
func TestGenerateAcceptsStringStatusCode(t *testing.T) {
	body := `{"base_resp":{"status_code":"1008","status_msg":"insufficient balance"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
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
	if !apiErr.IsInsufficientBalance() {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, StatusInsufficientBalance)
	}
}

// TestGenerateDecodeFailureIsNotInvalidParams verifies that a body this
// client cannot read is not reported as an invalid parameter error.
//
// MiniMax did not reject the parameters in that case, so the message
// must not send the reader off to adjust the generation settings.
func TestGenerateDecodeFailureIsNotInvalidParams(t *testing.T) {
	body := "<html><head><title>502 Bad Gateway</title></head></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
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
	if apiErr.IsInvalidParams() {
		t.Error("an unreadable response must not report itself as invalid params")
	}
	if !apiErr.IsUnreadableResponse() {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, StatusUnreadableResponse)
	}
	if apiErr.Raw != body {
		t.Errorf("raw body = %q, want %q", apiErr.Raw, body)
	}
	if !strings.Contains(apiErr.Message, "502 Bad Gateway") {
		t.Errorf("message %q does not quote the body", apiErr.Message)
	}
	if !strings.Contains(apiErr.HumanMessage(), "debug console") {
		t.Errorf("human message %q does not point at the raw body", apiErr.HumanMessage())
	}
}

// TestPingDecodeFailureIsNotInvalidParams verifies the same for the
// connection test.
func TestPingDecodeFailureIsNotInvalidParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	err := client.Ping(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *minimax.Error", err)
	}
	if apiErr.IsInvalidParams() {
		t.Error("an unreadable response must not report itself as invalid params")
	}
	if !apiErr.IsUnreadableResponse() {
		t.Errorf("status = %d, want %d", apiErr.StatusCode, StatusUnreadableResponse)
	}
}

// TestExcerptCutsOnARuneBoundary verifies that a shortened body never
// ends in half of a multi-byte character.
func TestExcerptCutsOnARuneBoundary(t *testing.T) {
	// A body of multi-byte characters, cut in the middle of one.
	body := strings.Repeat("日", 100)

	got := excerpt(body, 5)
	if !utf8.ValidString(got) {
		t.Errorf("excerpt produced invalid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("excerpt %q does not end with the ellipsis", got)
	}
}

// TestGenerateKeepsTheRawResponseBody verifies that the raw response
// body is kept on the error, so that the debug console can show exactly
// what the API returned.
func TestGenerateKeepsTheRawResponseBody(t *testing.T) {
	body := `{"id":"3c7b1f9e2a","base_resp":{"status_code":2013,"status_msg":"invalid parameter"},"data":{"image_base64":[]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
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
	if apiErr.Raw == "" {
		t.Fatal("the raw response body was not kept")
	}
	if apiErr.Raw != body {
		t.Errorf("raw body = %q, want %q", apiErr.Raw, body)
	}
}

// TestGenerateKeepsTheRawBodyOnAnHTTPFailure verifies that the body is
// kept when the transport fails, which is where the useful detail is.
func TestGenerateKeepsTheRawBodyOnAnHTTPFailure(t *testing.T) {
	body := `{"base_resp":{"status_code":1004,"status_msg":"invalid api key"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(body))
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
	if apiErr.Raw != body {
		t.Errorf("raw body = %q, want %q", apiErr.Raw, body)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("http status = %d, want %d", apiErr.HTTPStatus, http.StatusUnauthorized)
	}
}
