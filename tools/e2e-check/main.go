// A live end-to-end check: submit a job against a stub MiniMax server
// and confirm the image is persisted and served by PocketBase's file
// route.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/incurl/aurore/jobs"
	"github.com/incurl/aurore/minimax"
	aurorepb "github.com/incurl/aurore/pocketbase"
)

func main() {
	dir, err := os.MkdirTemp("", "aurore-e2e")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	svc := aurorepb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		panic(err)
	}
	fmt.Println("1. PocketBase bootstrapped, schema applied")

	// A stub MiniMax that returns one real 1x1 PNG.
	png := []byte{
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

	var sent map[string]any
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(minimax.ImageResponse{
			Data:     minimax.DataObject{ImageBase64: []string{base64.StdEncoding.EncodeToString(png)}},
			Metadata: minimax.Metadata{SuccessCount: 1},
			ID:       "e2e-task-1",
			BaseResp: minimax.BaseResp{StatusCode: minimax.StatusOK, StatusMsg: "success"},
		})
	}))
	defer stub.Close()

	manager := jobs.NewManager(svc.App(),
		jobs.WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("test-key",
				minimax.WithBaseURL(stub.URL),
				minimax.WithHTTPClient(stub.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), jobs.Request{
		Prompt:         "a red bicycle",
		NegativePrompt: "blurry",
		Params: jobs.Params{
			Model:            "image-01",
			AspectRatio:      "16:9",
			BatchSize:        1,
			UseNegativePrompt: true,
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("2. Job submitted: %s (status %s)\n", job.ID, job.Status)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := manager.GetJob(job.ID)
		if current.Status == "completed" || current.Status == "failed" {
			job = current
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("3. Job finished: status=%s taskID=%s images=%d\n", job.Status, job.MiniMaxTaskID, len(job.ImageIDs))
	if job.Status != "completed" {
		fmt.Println("   error:", job.ErrorMessage)
		os.Exit(1)
	}

	images, err := manager.ListImages(job.ID, 10)
	if err != nil {
		panic(err)
	}
	image := images[0]
	fmt.Printf("4. Image persisted: %s (%dx%d, %d bytes)\n", image.URL, image.Width, image.Height, image.Bytes)
	fmt.Printf("   thumbnail: %s\n", image.ThumbnailURL)

	// The file must be served by PocketBase's native route.
	handler, err := svc.Handler()
	if err != nil {
		panic(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, image.URL, nil))
	fmt.Printf("5. File route: GET %s -> %d (%d bytes, %s)\n", image.URL, rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	if rec.Code != 200 || rec.Body.Len() == 0 {
		os.Exit(1)
	}

	fmt.Printf("6. Request sent to MiniMax: prompt=%q n=%v aspect_ratio=%v\n",
		sent["prompt"], sent["n"], sent["aspect_ratio"])
	if _, ok := sent["negative_prompt"]; ok {
		fmt.Println("   FAIL: negative_prompt reached the API")
		os.Exit(1)
	}
	fmt.Println("   negative_prompt correctly absent from the API request")
	fmt.Println("\nEND TO END: PASS")
}
