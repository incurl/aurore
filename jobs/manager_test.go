package jobs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/incurl/aurore/minimax"
	aurorepb "github.com/incurl/aurore/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// newTestApp bootstraps an isolated PocketBase instance with the Aurore
// schema applied.
func newTestApp(t *testing.T) core.App {
	t.Helper()

	dir := t.TempDir()
	svc := aurorepb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return svc.App()
}

// pngBytes returns a tiny valid PNG image.
func pngBytes(t *testing.T, r, g, b uint8) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata_sample.png")
	if err == nil {
		return data
	}
	// Fall back to a 1x1 PNG built by hand.
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
		0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

// fakeMiniMax starts a stub of the MiniMax image generation endpoint.
func fakeMiniMax(t *testing.T, statusCode int, images int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if statusCode != minimax.StatusOK {
			_ = json.NewEncoder(w).Encode(minimax.ImageResponse{
				BaseResp: minimax.BaseResp{StatusCode: statusCode, StatusMsg: "stubbed failure"},
			})
			return
		}

		encoded := make([]string, 0, images)
		for i := 0; i < images; i++ {
			encoded = append(encoded, base64.StdEncoding.EncodeToString(pngBytes(t, 0, 0, 0)))
		}

		_ = json.NewEncoder(w).Encode(minimax.ImageResponse{
			Data:     minimax.DataObject{ImageBase64: encoded},
			Metadata: minimax.Metadata{SuccessCount: images},
			ID:       "task-abc",
			BaseResp: minimax.BaseResp{StatusCode: minimax.StatusOK, StatusMsg: "success"},
		})
	}))
}

// TestSubmitJobSucceedsWithTheDocumentedResponse verifies that a job
// completes when MiniMax answers in the format its documentation shows.
//
// The published example returns the image counts as JSON strings, so the
// response failed to decode and the job failed with "invalid parameters"
// even though the API had generated the images.
func TestSubmitJobSucceedsWithTheDocumentedResponse(t *testing.T) {
	app := newTestApp(t)

	encoded := base64.StdEncoding.EncodeToString(pngBytes(t, 0, 0, 0))
	body := `{"id":"task-documented","data":{"image_base64":["` + encoded + `"]},` +
		`"metadata":{"failed_count":"0","success_count":"1"},` +
		`"base_resp":{"status_code":0,"status_msg":"success"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	done := waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted, aurorepb.StatusFailed)
	if done.Status != aurorepb.StatusCompleted {
		t.Fatalf("status = %q, want completed (error %d: %s)",
			done.Status, done.ErrorCode, done.ErrorMessage)
	}
	if done.MiniMaxTaskID != "task-documented" {
		t.Errorf("task id = %q, want task-documented", done.MiniMaxTaskID)
	}
	if len(done.ImageIDs) != 1 {
		t.Errorf("image ids = %v, want one image", done.ImageIDs)
	}
}

// waitForStatus polls until the job reaches one of the wanted statuses.
func waitForStatus(t *testing.T, manager *Manager, jobID string, wanted ...string) Job {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		job, err := manager.GetJob(jobID)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		for _, status := range wanted {
			if job.Status == status {
				return job
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	job, _ := manager.GetJob(jobID)
	t.Fatalf("job %q never reached %v (last status %q)", jobID, wanted, job.Status)
	return Job{}
}

// TestSubmitJobCompletes verifies the full happy path: a job is created,
// the MiniMax API is called, the image is stored as a file attachment and
// the job is marked completed.
func TestSubmitJobCompletes(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 2)
	defer srv.Close()

	var captured map[string]any
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer proxy.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(proxy.URL),
				minimax.WithHTTPClient(proxy.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{
			Model:       "image-01",
			AspectRatio: "1:1",
			BatchSize:   2,
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.Status != aurorepb.StatusPending {
		t.Errorf("initial status = %q, want pending", job.Status)
	}

	done := waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)
	if done.MiniMaxTaskID != "task-abc" {
		t.Errorf("task id = %q, want task-abc", done.MiniMaxTaskID)
	}
	if len(done.ImageIDs) != 2 {
		t.Errorf("image count = %d, want 2", len(done.ImageIDs))
	}

	// The request sent to MiniMax must carry the prompt and the batch
	// size, and must not carry a negative prompt or a mask.
	if captured["prompt"] != "a red bicycle" {
		t.Errorf("sent prompt = %v", captured["prompt"])
	}
	if captured["n"] != float64(2) {
		t.Errorf("sent n = %v, want 2", captured["n"])
	}
	for _, forbidden := range []string{"negative_prompt", "mask", "mask_asset_id"} {
		if _, ok := captured[forbidden]; ok {
			t.Errorf("the request must not contain %q", forbidden)
		}
	}

	// The images must exist as generated_images records with files.
	images, err := manager.ListImages(job.ID, 10)
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("stored images = %d, want 2", len(images))
	}
	for _, image := range images {
		if image.URL == "" {
			t.Error("the image URL must be set")
		}
		if image.Bytes <= 0 {
			t.Errorf("image bytes = %d, want > 0", image.Bytes)
		}
	}
}

// TestSubmitJobFails verifies that a MiniMax error is recorded on the job
// with a human readable message.
func TestSubmitJobFails(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusInsufficientBalance, 0)
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, manager, job.ID, aurorepb.StatusFailed)
	if failed.ErrorCode != minimax.StatusInsufficientBalance {
		t.Errorf("error code = %d, want %d", failed.ErrorCode, minimax.StatusInsufficientBalance)
	}
	if failed.ErrorMessage == "" {
		t.Error("the error message must be set")
	}
}

// TestSubmitJobRecordsAnUnreadableResponse verifies that a response
// this client cannot read is recorded with the Aurore-side code, not
// with the invalid parameters code.
//
// The code is negative, so it also has to survive the round trip
// through the text field the job record stores it in.
func TestSubmitJobRecordsAnUnreadableResponse(t *testing.T) {
	app := newTestApp(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>gateway timeout</body></html>"))
	}))
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, manager, job.ID, aurorepb.StatusFailed)
	if failed.ErrorCode != minimax.StatusUnreadableResponse {
		t.Errorf("error code = %d, want %d", failed.ErrorCode, minimax.StatusUnreadableResponse)
	}
	if failed.ErrorCode == minimax.StatusInvalidParams {
		t.Error("an unreadable response must not be recorded as invalid params")
	}
	if !strings.Contains(failed.ErrorMessage, "could not be read") {
		t.Errorf("error message %q does not explain the failure", failed.ErrorMessage)
	}
}

// TestDeleteJobRemovesAFinishedJob verifies that a completed job can be
// removed from the queue, together with the images it produced.
func TestDeleteJobRemovesAFinishedJob(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 2)
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 2},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	done := waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)
	if len(done.ImageIDs) != 2 {
		t.Fatalf("image ids = %v, want two", done.ImageIDs)
	}

	if err := manager.DeleteJob(job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}

	// The job must be gone.
	if _, err := manager.GetJob(job.ID); err == nil {
		t.Error("the job is still there after it was deleted")
	}

	// So must its images, or they would point at a job that is not
	// there any more.
	images, err := manager.ListImages(job.ID, 10)
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("images = %d, want none", len(images))
	}
}

// TestDeleteJobRemovesAFailedJob verifies that a failed job can be
// removed too.
func TestDeleteJobRemovesAFailedJob(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusInsufficientBalance, 0)
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusFailed)

	if err := manager.DeleteJob(job.ID); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, err := manager.GetJob(job.ID); err == nil {
		t.Error("the job is still there after it was deleted")
	}
}

// TestDeleteJobRefusesARunningJob verifies that a job that has not come
// to rest cannot be deleted.
//
// It has to be cancelled first, so that it cannot write to the database
// after it has been removed.
func TestDeleteJobRefusesARunningJob(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 1)
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// The job is pending or running at this point.
	err = manager.DeleteJob(job.ID)
	if err == nil {
		t.Fatal("expected an error when deleting a job that has not finished")
	}
	if !strings.Contains(err.Error(), "cancel") {
		t.Errorf("error %q does not say that the job has to be cancelled first", err)
	}

	// The job must still be there.
	if _, err := manager.GetJob(job.ID); err != nil {
		t.Errorf("the job was removed even though the delete failed: %v", err)
	}
}

// TestDeleteUnknownJobFails verifies that deleting a job that is not
// there reports an error.
func TestDeleteUnknownJobFails(t *testing.T) {
	app := newTestApp(t)
	manager := NewManager(app)

	if err := manager.DeleteJob("nosuchjob"); err == nil {
		t.Fatal("expected an error when deleting a job that does not exist")
	}
}

// TestSubmitJobFoldsNegativePrompt verifies that the negative prompt is
// appended to the prompt only when the user opted in.
func TestSubmitJobFoldsNegativePrompt(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 1)
	defer srv.Close()

	var captured map[string]any
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer proxy.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(proxy.URL),
				minimax.WithHTTPClient(proxy.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt:         "a red bicycle",
		NegativePrompt: "blurry",
		Params:         Params{Model: "image-01", BatchSize: 1, UseNegativePrompt: true},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)

	prompt, _ := captured["prompt"].(string)
	if prompt != "a red bicycle Avoid: blurry." {
		t.Errorf("sent prompt = %q", prompt)
	}
}

// TestSubmitJobWithoutNegativePromptToggle verifies that the negative
// prompt is not sent when the toggle is off.
func TestSubmitJobWithoutNegativePromptToggle(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 1)
	defer srv.Close()

	var captured map[string]any
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		srv.Config.Handler.ServeHTTP(w, r)
	}))
	defer proxy.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(proxy.URL),
				minimax.WithHTTPClient(proxy.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt:         "a red bicycle",
		NegativePrompt: "blurry",
		Params:         Params{Model: "image-01", BatchSize: 1, UseNegativePrompt: false},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)

	if prompt, _ := captured["prompt"].(string); prompt != "a red bicycle" {
		t.Errorf("sent prompt = %q, want the positive prompt only", prompt)
	}
}

// TestSubmitJobRequiresPrompt verifies the input validation.
func TestSubmitJobRequiresPrompt(t *testing.T) {
	app := newTestApp(t)
	manager := NewManager(app)

	if _, err := manager.Submit(context.Background(), Request{}); err == nil {
		t.Fatal("expected an error for an empty prompt")
	}
}

// TestPresetsRoundTrip verifies saving, listing and deleting presets.
func TestPresetsRoundTrip(t *testing.T) {
	app := newTestApp(t)
	manager := NewManager(app)

	saved, err := manager.SavePreset(Preset{
		Name:   "portrait",
		Prompt: "a portrait",
		Params: Params{Model: "image-01", BatchSize: 2},
	})
	if err != nil {
		t.Fatalf("SavePreset: %v", err)
	}
	if saved.Name != "portrait" {
		t.Errorf("name = %q", saved.Name)
	}

	presets, err := manager.ListPresets()
	if err != nil {
		t.Fatalf("ListPresets: %v", err)
	}
	if len(presets) != 1 {
		t.Fatalf("presets = %d, want 1", len(presets))
	}
	if presets[0].Params.BatchSize != 2 {
		t.Errorf("batch size = %d, want 2", presets[0].Params.BatchSize)
	}

	if err := manager.DeletePreset(saved.ID); err != nil {
		t.Fatalf("DeletePreset: %v", err)
	}
	presets, _ = manager.ListPresets()
	if len(presets) != 0 {
		t.Errorf("presets = %d, want 0", len(presets))
	}
}

// TestUploadAsset verifies that an uploaded asset is stored as a file
// attachment and can be read back.
func TestUploadAsset(t *testing.T) {
	app := newTestApp(t)
	manager := NewManager(app)

	asset, err := manager.UploadAsset("reference", pngBytes(t, 0, 0, 0), "ref.png")
	if err != nil {
		t.Fatalf("UploadAsset: %v", err)
	}
	if asset.Kind != "reference" {
		t.Errorf("kind = %q", asset.Kind)
	}
	if asset.URL == "" {
		t.Error("the asset URL must be set")
	}

	data, err := manager.AssetBytes(asset.ID)
	if err != nil {
		t.Fatalf("AssetBytes: %v", err)
	}
	if len(data) == 0 {
		t.Error("the stored asset is empty")
	}
}

// TestUploadAssetRejectsUnknownKind verifies the kind validation.
func TestUploadAssetRejectsUnknownKind(t *testing.T) {
	app := newTestApp(t)
	manager := NewManager(app)

	if _, err := manager.UploadAsset("bogus", []byte{1}, "x.png"); err == nil {
		t.Fatal("expected an error for an unknown asset kind")
	}
}

// TestGeneratedImageIsStoredWithItsRealFormat verifies that a stored
// image is named after the format its bytes really are.
//
// MiniMax returns JPEG images. Naming them ".png" and declaring them
// image/png produced files that tools misidentify, and the export
// downloaded them under a ".png" name too.
func TestGeneratedImageIsStoredWithItsRealFormat(t *testing.T) {
	app := newTestApp(t)

	// A stub that answers with a real JPEG, which is what MiniMax sends.
	encoded := base64.StdEncoding.EncodeToString(jpegBytes(t))
	body := `{"id":"task-jpeg","data":{"image_base64":["` + encoded + `"]},` +
		`"metadata":{"failed_count":"0","success_count":"1"},` +
		`"base_resp":{"status_code":0,"status_msg":"success"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)

	images, err := manager.ListImages(job.ID, 10)
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}

	// The stored file must carry the real extension.
	record, err := app.FindRecordById("generated_images", images[0].ID)
	if err != nil {
		t.Fatalf("find image: %v", err)
	}
	stored := record.GetString("file")
	if !strings.HasSuffix(stored, ".jpg") {
		t.Errorf("stored file %q does not end in .jpg", stored)
	}
	if got := record.GetString("mime"); got != "image/jpeg" {
		t.Errorf("mime = %q, want image/jpeg", got)
	}
	if got := images[0].Mime; got != "image/jpeg" {
		t.Errorf("image mime = %q, want image/jpeg", got)
	}

	// The export must report the real format as well.
	exported, err := manager.ExportImage(images[0].ID)
	if err != nil {
		t.Fatalf("ExportImage: %v", err)
	}
	if exported.Extension != "jpg" {
		t.Errorf("export extension = %q, want jpg", exported.Extension)
	}
	if exported.Mime != "image/jpeg" {
		t.Errorf("export mime = %q, want image/jpeg", exported.Mime)
	}
	if len(exported.Bytes) == 0 {
		t.Error("the export returned no bytes")
	}

	// The bytes must be the JPEG that was stored, not a re-encoded PNG.
	if !bytes.HasPrefix(exported.Bytes, []byte{0xFF, 0xD8, 0xFF}) {
		t.Error("the exported bytes are not a JPEG")
	}
}

// TestGeneratedImageURLsAreServable verifies that the URLs handed to the
// frontend resolve through PocketBase's native file route.
//
// This is the contract the gallery relies on: the image URL must point at
// /api/files/<collection>/<record>/<stored filename>.
func TestGeneratedImageURLsAreServable(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 1)
	defer srv.Close()

	manager := NewManager(app,
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)

	images, err := manager.ListImages(job.ID, 10)
	if err != nil {
		t.Fatalf("ListImages: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}

	image := images[0]
	if !strings.HasPrefix(image.URL, "/api/files/generated_images/"+image.ID+"/") {
		t.Errorf("image URL = %q, want the native file route", image.URL)
	}
	if image.ThumbnailURL == "" {
		t.Error("the thumbnail URL must be set")
	}
	if !strings.HasPrefix(image.ThumbnailURL, "/api/files/generated_images/"+image.ID+"/") {
		t.Errorf("thumbnail URL = %q, want the native file route", image.ThumbnailURL)
	}

	// The stored filename must include the record id in the path.
	record, err := app.FindRecordById("generated_images", image.ID)
	if err != nil {
		t.Fatalf("find image: %v", err)
	}
	if record.GetString("file") == "" {
		t.Error("the image record has no stored file")
	}
	if record.GetString("thumbnail") == "" {
		t.Error("the image record has no thumbnail")
	}
}
