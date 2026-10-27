package minimax

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPayloadCoreFields(t *testing.T) {
	req := ImageRequest{
		Model:  ModelImage01,
		Prompt: "a red bicycle",
		N:      3,
		Seed:   42,
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	if got := payload["model"]; got != "image-01" {
		t.Errorf("model = %v, want image-01", got)
	}
	if got := payload["prompt"]; got != "a red bicycle" {
		t.Errorf("prompt = %v", got)
	}
	if got := payload["n"]; got != 3 {
		t.Errorf("n = %v, want 3", got)
	}
	if got := payload["seed"]; got != int64(42) {
		t.Errorf("seed = %v, want 42", got)
	}
}

func TestBuildPayloadNeverSendsUnsupportedFields(t *testing.T) {
	req := ImageRequest{
		Model:          ModelImage01,
		Prompt:         "hello",
		NegativePrompt: "blurry, ugly",
		MaskAssetID:    "abc123",
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	for _, forbidden := range []string{"negative_prompt", "mask_asset_id", "mask", "cfg_scale", "sampler", "steps", "scheduler", "lora", "controlnet"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("payload must not contain unsupported field %q", forbidden)
		}
	}
}

func TestBuildPayloadFoldsNegativePrompt(t *testing.T) {
	req := ImageRequest{
		Model:          ModelImage01,
		Prompt:         "a red bicycle",
		NegativePrompt: "blurry",
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	prompt, _ := payload["prompt"].(string)
	if !strings.Contains(prompt, NegativePromptPrefix+"blurry") {
		t.Errorf("prompt %q should contain the folded negative prompt", prompt)
	}
	if !strings.HasPrefix(prompt, "a red bicycle") {
		t.Errorf("prompt %q should retain the original positive prompt", prompt)
	}
}

func TestBuildPayloadTruncatesLongPrompt(t *testing.T) {
	req := ImageRequest{Model: ModelImage01, Prompt: strings.Repeat("x", 2000)}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	prompt, _ := payload["prompt"].(string)
	if len(prompt) != MaxPromptLength {
		t.Errorf("prompt length = %d, want %d", len(prompt), MaxPromptLength)
	}
}

func TestBuildPayloadRejectsEmptyPrompt(t *testing.T) {
	if _, err := BuildPayload(ImageRequest{Model: ModelImage01, Prompt: "   "}); err == nil {
		t.Fatal("expected an error for an empty prompt")
	}
}

func TestBuildPayloadRejectsUnknownModel(t *testing.T) {
	if _, err := BuildPayload(ImageRequest{Model: "image-99", Prompt: "x"}); err == nil {
		t.Fatal("expected an error for an unsupported model")
	}
}

func TestBuildPayloadClampsBatchSize(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 1},
		{1, 1},
		{5, 5},
		{9, 9},
		{10, 9},
		{-3, 1},
	}
	for _, tc := range cases {
		payload, err := BuildPayload(ImageRequest{Model: ModelImage01, Prompt: "x", N: tc.in})
		if err != nil {
			t.Fatalf("BuildPayload(n=%d): %v", tc.in, err)
		}
		if got := payload["n"]; got != tc.want {
			t.Errorf("n(%d) = %v, want %d", tc.in, got, tc.want)
		}
	}
}

func TestBuildPayloadOmitsZeroSeed(t *testing.T) {
	payload, err := BuildPayload(ImageRequest{Model: ModelImage01, Prompt: "x"})
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	if _, ok := payload["seed"]; ok {
		t.Error("seed must be omitted when zero so the server picks a random seed")
	}
}

func TestBuildPayloadDimensionsTakePrecedenceOverAspectRatio(t *testing.T) {
	req := ImageRequest{
		Model:       ModelImage01,
		Prompt:      "x",
		AspectRatio: AspectRatio16x9,
		Width:       1024,
		Height:      768,
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	if _, ok := payload["aspect_ratio"]; ok {
		t.Error("aspect_ratio must be dropped when valid width/height are supplied")
	}
	if payload["width"] != 1024 || payload["height"] != 768 {
		t.Errorf("dimensions = %v x %v, want 1024 x 768", payload["width"], payload["height"])
	}
}

func TestBuildPayloadFallsBackToAspectRatioForInvalidDimensions(t *testing.T) {
	cases := []struct{ w, h int }{
		{500, 500},   // below the minimum
		{3000, 1000}, // above the maximum
		{1023, 768},  // not a multiple of 8
		{1024, 0},    // incomplete pair
	}
	for _, tc := range cases {
		req := ImageRequest{
			Model:       ModelImage01,
			Prompt:      "x",
			AspectRatio: AspectRatio16x9,
			Width:       tc.w,
			Height:      tc.h,
		}
		payload, err := BuildPayload(req)
		if err != nil {
			t.Fatalf("BuildPayload(%d x %d): %v", tc.w, tc.h, err)
		}
		if _, ok := payload["width"]; ok {
			t.Errorf("%d x %d: invalid dimensions must be dropped", tc.w, tc.h)
		}
		if payload["aspect_ratio"] != "16:9" {
			t.Errorf("%d x %d: aspect_ratio = %v, want 16:9", tc.w, tc.h, payload["aspect_ratio"])
		}
	}
}

func TestBuildPayloadDropsDimensionsForLiveModel(t *testing.T) {
	req := ImageRequest{
		Model:       ModelImage01Live,
		Prompt:      "x",
		AspectRatio: AspectRatio16x9,
		Width:       1024,
		Height:      768,
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	for _, forbidden := range []string{"aspect_ratio", "width", "height"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("image-01-live must not receive %q", forbidden)
		}
	}
}

func TestBuildPayloadStyleOnlyForLiveModel(t *testing.T) {
	style := &StyleObject{StyleType: StyleWatercolor, StyleWeight: 0.8}

	payload, err := BuildPayload(ImageRequest{Model: ModelImage01, Prompt: "x", Style: style})
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	if _, ok := payload["style"]; ok {
		t.Error("image-01 must not receive a style preset")
	}

	payload, err = BuildPayload(ImageRequest{Model: ModelImage01Live, Prompt: "x", Style: style})
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	got, ok := payload["style"].(map[string]any)
	if !ok {
		t.Fatalf("style = %v, want a map", payload["style"])
	}
	if got["style_type"] != string(StyleWatercolor) {
		t.Errorf("style_type = %v, want %v", got["style_type"], StyleWatercolor)
	}
	if got["style_weight"] != 0.8 {
		t.Errorf("style_weight = %v, want 0.8", got["style_weight"])
	}
}

func TestBuildPayloadKeepsSingleReferenceImage(t *testing.T) {
	req := ImageRequest{
		Model:  ModelImage01,
		Prompt: "x",
		SubjectReference: []SubjectReference{
			{Type: SubjectReferenceCharacter, ImageFile: "data:image/png;base64,AAA"},
			{Type: SubjectReferenceCharacter, ImageFile: "data:image/png;base64,BBB"},
		},
	}
	payload, err := BuildPayload(req)
	if err != nil {
		t.Fatalf("BuildPayload: %v", err)
	}
	refs, ok := payload["subject_reference"].([]map[string]any)
	if !ok {
		t.Fatalf("subject_reference = %v, want a slice", payload["subject_reference"])
	}
	if len(refs) != 1 {
		t.Errorf("subject_reference length = %d, want 1", len(refs))
	}
	if refs[0]["type"] != "character" {
		t.Errorf("type = %v, want character", refs[0]["type"])
	}
}

func TestImageRequestMarshalOmitsLocalOnlyFields(t *testing.T) {
	req := ImageRequest{
		Model:          ModelImage01,
		Prompt:         "hello",
		NegativePrompt: "ugly",
		MaskAssetID:    "mask-1",
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(raw)
	for _, forbidden := range []string{"negative_prompt", "mask_asset_id"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("marshalled request must not contain %q: %s", forbidden, s)
		}
	}
}

func TestSupportsDimensionsAndStyle(t *testing.T) {
	if !SupportsDimensions(ModelImage01) {
		t.Error("image-01 must support dimensions")
	}
	if SupportsDimensions(ModelImage01Live) {
		t.Error("image-01-live must not support dimensions")
	}
	if SupportsStyle(ModelImage01) {
		t.Error("image-01 must not support style presets")
	}
	if !SupportsStyle(ModelImage01Live) {
		t.Error("image-01-live must support style presets")
	}
}
