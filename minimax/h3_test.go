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

// TestBuildH3PayloadTextToImage verifies the body and the endpoint for a
// plain text-to-image request.
func TestBuildH3PayloadTextToImage(t *testing.T) {
	path, payload, err := BuildH3Payload(ImageRequest{
		Model:       ModelH3Image,
		Prompt:      "an orange cat",
		N:           3,
		AspectRatio: AspectRatio16x9,
		Area:        Area4K,
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if path != H3GenerationsPath {
		t.Errorf("path = %q, want %q", path, H3GenerationsPath)
	}
	if payload["model"] != string(ModelH3Image) {
		t.Errorf("model = %v", payload["model"])
	}
	if payload["prompt"] != "an orange cat" {
		t.Errorf("prompt = %v", payload["prompt"])
	}
	if payload["n"] != 3 {
		t.Errorf("n = %v, want 3", payload["n"])
	}
	if payload["response_format"] != "b64_json" {
		t.Errorf("response_format = %v, want b64_json", payload["response_format"])
	}
	if payload["aspect_ratio"] != string(AspectRatio16x9) {
		t.Errorf("aspect_ratio = %v", payload["aspect_ratio"])
	}
	if payload["area"] != string(Area4K) {
		t.Errorf("area = %v", payload["area"])
	}
	// The prompt optimiser is expressed inverted.
	if payload["disable_prompt_enhanced"] != true {
		t.Errorf("disable_prompt_enhanced = %v, want true", payload["disable_prompt_enhanced"])
	}

	// H3 has no seed, style or watermark, so none may appear on the wire.
	for _, field := range []string{"seed", "style", "aigc_watermark", "width", "height", "images"} {
		if _, ok := payload[field]; ok {
			t.Errorf("the payload must not carry %q", field)
		}
	}
}

// TestBuildH3PayloadPromptOptimizerIsInverted verifies the inverse flag.
func TestBuildH3PayloadPromptOptimizerIsInverted(t *testing.T) {
	_, payload, err := BuildH3Payload(ImageRequest{
		Model:           ModelH3Image,
		Prompt:          "x",
		PromptOptimizer: true,
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if payload["disable_prompt_enhanced"] != false {
		t.Errorf("disable_prompt_enhanced = %v, want false", payload["disable_prompt_enhanced"])
	}
}

// TestBuildH3PayloadUsesTheEditEndpointForReferences verifies that a
// reference image selects the edit endpoint and travels as a data URI.
func TestBuildH3PayloadUsesTheEditEndpointForReferences(t *testing.T) {
	dataURL := "data:image/jpeg;base64,AAAA"
	path, payload, err := BuildH3Payload(ImageRequest{
		Model:  ModelH3Image,
		Prompt: "make it steampunk",
		SubjectReference: []SubjectReference{
			{Type: SubjectReferenceCharacter, ImageFile: dataURL},
		},
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if path != H3EditsPath {
		t.Errorf("path = %q, want %q", path, H3EditsPath)
	}
	images, ok := payload["images"].([]map[string]any)
	if !ok || len(images) != 1 {
		t.Fatalf("images = %v, want one entry", payload["images"])
	}
	if images[0]["image_url"] != dataURL {
		t.Errorf("image_url = %v, want the data URL", images[0]["image_url"])
	}
	// The subject_reference field belongs to Image-01 and must not leak.
	if _, ok := payload["subject_reference"]; ok {
		t.Error("the payload must not carry subject_reference")
	}
}

// TestBuildH3PayloadSizesWithRatioAndArea verifies that dimensions are
// dropped in favour of the ratio and area, which the API always accepts.
func TestBuildH3PayloadSizesWithRatioAndArea(t *testing.T) {
	// 512x512 is below the H3 minimum area, so it must not be sent as a
	// size: the API would reject it rather than scale it.
	_, payload, err := BuildH3Payload(ImageRequest{
		Model: ModelH3Image, Prompt: "x",
		Width: 512, Height: 512,
		AspectRatio: AspectRatio1x1,
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if _, ok := payload["size"]; ok {
		t.Errorf("size = %v, want it omitted for an out-of-range pair", payload["size"])
	}
	if payload["aspect_ratio"] != string(AspectRatio1x1) {
		t.Errorf("aspect_ratio = %v", payload["aspect_ratio"])
	}
	if payload["area"] != string(DefaultArea) {
		t.Errorf("area = %v, want the default %q", payload["area"], DefaultArea)
	}
}

// TestBuildH3PayloadSendsAValidExplicitSize verifies that a size inside
// the H3 limits is sent as WIDTHxHEIGHT.
func TestBuildH3PayloadSendsAValidExplicitSize(t *testing.T) {
	_, payload, err := BuildH3Payload(ImageRequest{
		Model: ModelH3Image, Prompt: "x",
		Width: 2048, Height: 1152,
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if payload["size"] != "2048x1152" {
		t.Errorf("size = %v, want 2048x1152", payload["size"])
	}
	if _, ok := payload["aspect_ratio"]; ok {
		t.Error("aspect_ratio must not accompany an explicit size")
	}
	if _, ok := payload["area"]; ok {
		t.Error("area must not accompany an explicit size")
	}
}

// TestBuildH3PayloadClampsTheBatch verifies the larger H3 batch range.
func TestBuildH3PayloadClampsTheBatch(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 1}, {1, 1}, {10, 10}, {50, H3MaxBatchSize},
	}
	for _, tc := range cases {
		_, payload, err := BuildH3Payload(ImageRequest{
			Model: ModelH3Image, Prompt: "x", N: tc.in,
		})
		if err != nil {
			t.Fatalf("BuildH3Payload: %v", err)
		}
		if payload["n"] != tc.want {
			t.Errorf("n for input %d = %v, want %d", tc.in, payload["n"], tc.want)
		}
	}
}

// TestBuildH3PayloadFoldsTheNegativePrompt verifies that the negative
// prompt reaches the model, which has no field for it.
func TestBuildH3PayloadFoldsTheNegativePrompt(t *testing.T) {
	_, payload, err := BuildH3Payload(ImageRequest{
		Model: ModelH3Image, Prompt: "a red bicycle", NegativePrompt: "blurry",
	})
	if err != nil {
		t.Fatalf("BuildH3Payload: %v", err)
	}
	if !strings.Contains(payload["prompt"].(string), "Avoid: blurry") {
		t.Errorf("prompt = %v, want the folded negative prompt", payload["prompt"])
	}
}

// TestBuildH3PayloadRejectsNonH3Models verifies the guard.
func TestBuildH3PayloadRejectsNonH3Models(t *testing.T) {
	if _, _, err := BuildH3Payload(ImageRequest{Model: ModelImage01, Prompt: "x"}); err == nil {
		t.Fatal("expected an error for a non-H3 model")
	}
}

// TestBuildH3PayloadRequiresAPrompt verifies the guard.
func TestBuildH3PayloadRequiresAPrompt(t *testing.T) {
	if _, _, err := BuildH3Payload(ImageRequest{Model: ModelH3Image, Prompt: "   "}); err == nil {
		t.Fatal("expected an error for an empty prompt")
	}
}

// TestH3ErrorEnvelopeExtractsTheCode verifies that the numeric code is
// recovered from the parenthesised suffix H3 uses.
func TestH3ErrorEnvelopeExtractsTheCode(t *testing.T) {
	body := `{"type":"error","error":{"type":"authorized_error","message":"login fail: Please carry the API secret key in the 'Authorization' field of the request header (1004)","http_code":"401"},"request_id":"abc"}`

	var env h3ErrorEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := env.statusCode(); got != StatusAuthFailed {
		t.Errorf("statusCode = %d, want %d", got, StatusAuthFailed)
	}
	if got := env.httpStatus(); got != 401 {
		t.Errorf("httpStatus = %d, want 401", got)
	}
}

// TestH3ErrorEnvelopeWithoutACode verifies that a message with no
// parenthesised code yields zero rather than a wrong code.
func TestH3ErrorEnvelopeWithoutACode(t *testing.T) {
	var env h3ErrorEnvelope
	env.Error.Message = "something went wrong"
	if got := env.statusCode(); got != 0 {
		t.Errorf("statusCode = %d, want 0", got)
	}
}

// TestGenerateH3DecodesB64Images verifies the whole H3 path against a
// stub: the endpoint, the body it is sent, and the decoded images.
func TestGenerateH3DecodesB64Images(t *testing.T) {
	first := base64.StdEncoding.EncodeToString([]byte("image-one"))
	second := base64.StdEncoding.EncodeToString([]byte("image-two"))

	var gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"created":1,"data":[{"b64_json":%q,"size":"2720x1536"},{"b64_json":%q,"size":"2720x1536"}],"output_format":"jpeg","usage":{"generated_images":2},"request_id":"req-1"}`,
			first, second)
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	res, err := client.Generate(context.Background(), ImageRequest{
		Model: ModelH3Image, Prompt: "a cat", N: 2,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if gotPath != H3GenerationsPath {
		t.Errorf("path = %q, want %q", gotPath, H3GenerationsPath)
	}
	if gotBody["model"] != string(ModelH3Image) {
		t.Errorf("sent model = %v", gotBody["model"])
	}
	if gotBody["response_format"] != "b64_json" {
		t.Errorf("sent response_format = %v", gotBody["response_format"])
	}
	if len(res.Images) != 2 {
		t.Fatalf("images = %d, want 2", len(res.Images))
	}
	if string(res.Images[0]) != "image-one" || string(res.Images[1]) != "image-two" {
		t.Errorf("images = %q", res.Images)
	}
	if res.TaskID != "req-1" {
		t.Errorf("request id = %q, want req-1", res.TaskID)
	}
	if res.SuccessCount != 2 {
		t.Errorf("success count = %d, want 2", res.SuccessCount)
	}
}

// TestGenerateH3CountsPartialFailures verifies that an image withheld by
// content safety is counted rather than treated as a failure of the
// whole request.
func TestGenerateH3CountsPartialFailures(t *testing.T) {
	good := base64.StdEncoding.EncodeToString([]byte("kept"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[{"b64_json":%q},{"error":{"code":"1027","message":"output image failed content safety check"}}],"usage":{"generated_images":1},"request_id":"r"}`, good)
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	res, err := client.Generate(context.Background(), ImageRequest{
		Model: ModelH3Image, Prompt: "a cat", N: 2,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res.Images) != 1 {
		t.Errorf("images = %d, want 1", len(res.Images))
	}
	if res.FailedCount != 1 {
		t.Errorf("failed count = %d, want 1", res.FailedCount)
	}
}

// TestGenerateH3ReportsTheErrorEnvelope verifies that the numeric code is
// recovered from the OpenAI-style error body, so the rest of Aurore can
// explain it in the same terms as an Image-01 failure.
func TestGenerateH3ReportsTheErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authorized_error","message":"login fail (1004)","http_code":"401"},"request_id":"r"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	_, err := client.Generate(context.Background(), ImageRequest{Model: ModelH3Image, Prompt: "a cat"})
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *minimax.Error", err)
	}
	if !apiErr.IsAuthError() {
		t.Errorf("status = %d, want an auth error", apiErr.StatusCode)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("http status = %d, want 401", apiErr.HTTPStatus)
	}
	if !strings.Contains(apiErr.HumanMessage(), "API key") {
		t.Errorf("human message %q does not explain the auth failure", apiErr.HumanMessage())
	}
}

// TestGenerateH3AllImagesRejectedFails verifies that a response with no
// usable image is an error rather than an empty success.
func TestGenerateH3AllImagesRejectedFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"error":{"code":"1027","message":"blocked"}}],"usage":{"generated_images":0}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv, "key")
	if _, err := client.Generate(context.Background(), ImageRequest{Model: ModelH3Image, Prompt: "a cat"}); err == nil {
		t.Fatal("expected an error when every image was rejected")
	}
}
