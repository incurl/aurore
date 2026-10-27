package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/incurl/aurore/minimax"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// EventEmitter is notified whenever a job changes state. The Wails layer
// implements it to push updates to the frontend.
type EventEmitter interface {
	// EmitJobUpdated publishes an updated job.
	EmitJobUpdated(job Job)
}

// Manager orchestrates generation jobs.
type Manager struct {
	app     core.App
	emitter EventEmitter

	// clientFactory builds a MiniMax client for the currently
	// configured credentials.
	clientFactory func(ctx context.Context) (*minimax.Client, error)

	// emitterFn lazily resolves the emitter, if one was configured.
	emitterFn func() EventEmitter

	// mu guards the cancellation map.
	mu sync.Mutex
	// cancels holds the cancel function of each running job.
	cancels map[string]context.CancelFunc

	// queue bounds how many jobs run at once.
	queue chan struct{}
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithEmitter sets the job update emitter.
func WithEmitter(e EventEmitter) ManagerOption {
	return func(m *Manager) { m.emitter = e }
}

// WithEmitterFunc sets a lazily resolved job update emitter. It is used
// because the emitter needs the Wails application, which is created
// after the job manager.
func WithEmitterFunc(fn func() EventEmitter) ManagerOption {
	return func(m *Manager) { m.emitterFn = fn }
}

// WithClientFactory sets the factory used to build MiniMax clients.
func WithClientFactory(f func(ctx context.Context) (*minimax.Client, error)) ManagerOption {
	return func(m *Manager) { m.clientFactory = f }
}

// WithConcurrency sets the number of jobs executed in parallel.
func WithConcurrency(n int) ManagerOption {
	return func(m *Manager) {
		if n <= 0 {
			n = 1
		}
		m.queue = make(chan struct{}, n)
	}
}

// NewManager creates a job manager.
func NewManager(app core.App, opts ...ManagerOption) *Manager {
	m := &Manager{
		app:     app,
		cancels: make(map[string]context.CancelFunc),
		queue:   make(chan struct{}, 2),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// App returns the underlying PocketBase app.
func (m *Manager) App() core.App { return m.app }

// SetEmitter attaches the job update emitter once it becomes available.
func (m *Manager) SetEmitter(e EventEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitter = e
}

// currentEmitter resolves the active emitter.
func (m *Manager) currentEmitter() EventEmitter {
	m.mu.Lock()
	emitter := m.emitter
	fn := m.emitterFn
	m.mu.Unlock()

	if emitter == nil && fn != nil {
		emitter = fn()
	}
	return emitter
}

// Submit creates a job record and schedules it for execution.
//
// The job is persisted with status "pending" before it is queued, so the
// UI can display it immediately.
func (m *Manager) Submit(ctx context.Context, req Request) (Job, error) {
	if req.Prompt == "" {
		return Job{}, fmt.Errorf("a prompt is required")
	}
	if req.Params.BatchSize <= 0 {
		req.Params.BatchSize = 1
	}
	if req.Params.BatchSize > 9 {
		req.Params.BatchSize = 9
	}
	if req.Params.Model == "" {
		req.Params.Model = string(minimax.ModelImage01)
	}
	if req.Params.MaskMode == "" {
		req.Params.MaskMode = MaskModeInside
	}

	col, err := m.app.FindCollectionByNameOrId("image_jobs")
	if err != nil {
		return Job{}, fmt.Errorf("find image_jobs collection: %w", err)
	}

	record := core.NewRecord(col)
	record.Set("prompt", req.Prompt)
	record.Set("negative_prompt", req.NegativePrompt)
	record.Set("status", "pending")
	record.Set("params", req.Params)
	record.Set("seed_requested", req.Params.Seed)
	record.Set("image_count", 0)

	if err := m.app.Save(record); err != nil {
		return Job{}, fmt.Errorf("create job: %w", err)
	}

	job, err := m.toJob(record)
	if err != nil {
		return Job{}, err
	}

	// Link the uploaded assets to the job now that it has an id.
	if err := m.linkAssets(record.Id, req); err != nil {
		return Job{}, err
	}

	go m.run(record.Id, req)

	return job, nil
}

// linkAssets binds the reference and mask uploads to the job.
func (m *Manager) linkAssets(jobID string, req Request) error {
	for _, assetID := range []string{req.ReferenceAssetID, req.MaskAssetID} {
		if assetID == "" {
			continue
		}
		record, err := m.app.FindRecordById("uploaded_assets", assetID)
		if err != nil {
			return fmt.Errorf("find asset %q: %w", assetID, err)
		}
		record.Set("job", jobID)
		if err := m.app.Save(record); err != nil {
			return fmt.Errorf("link asset %q: %w", assetID, err)
		}
	}
	return nil
}

// Cancel aborts a pending or running job.
func (m *Manager) Cancel(jobID string) error {
	m.mu.Lock()
	cancel, running := m.cancels[jobID]
	m.mu.Unlock()

	if running && cancel != nil {
		cancel()
	}

	record, err := m.app.FindRecordById("image_jobs", jobID)
	if err != nil {
		return fmt.Errorf("find job %q: %w", jobID, err)
	}

	status := record.GetString("status")
	if status == "completed" || status == "cancelled" {
		return nil
	}
	if status == "failed" {
		return fmt.Errorf("job %q already failed and cannot be cancelled", jobID)
	}

	record.Set("status", "cancelled")
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("cancel job %q: %w", jobID, err)
	}

	m.emitRecord(record)
	return nil
}

// run executes a job.
func (m *Manager) run(jobID string, req Request) {
	m.queue <- struct{}{}
	defer func() { <-m.queue }()

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[jobID] = cancel
	m.mu.Unlock()

	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.cancels, jobID)
		m.mu.Unlock()
	}()

	if err := m.execute(ctx, jobID, req); err != nil {
		m.fail(jobID, err)
	}
}

// execute performs the API call and persists the results.
func (m *Manager) execute(ctx context.Context, jobID string, req Request) error {
	record, err := m.app.FindRecordById("image_jobs", jobID)
	if err != nil {
		return fmt.Errorf("find job %q: %w", jobID, err)
	}

	// A job cancelled before it started must not call the API.
	if record.GetString("status") == "cancelled" {
		return nil
	}

	record.Set("status", "running")
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}
	m.emitRecord(record)

	client, err := m.clientFactory(ctx)
	if err != nil {
		return err
	}

	// Build the reference image data URL, if a reference was uploaded.
	referenceDataURL, err := m.assetDataURL(req.ReferenceAssetID)
	if err != nil {
		return err
	}

	apiReq := req.toMiniMaxRequest(referenceDataURL)
	result, err := client.Generate(ctx, apiReq)
	if err != nil {
		return err
	}

	// Persist the task id and the effective seed.
	record.Set("minimax_task_id", result.TaskID)
	record.Set("seed_effective", req.Params.Seed)
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("save task id: %w", err)
	}

	// The mask composite needs the original image, which is the source
	// of the reference or the previous generation. Without one, only the
	// generated image is available.
	var originalBytes []byte
	if req.MaskAssetID != "" {
		originalBytes, _ = m.assetBytes(req.ReferenceAssetID)
	}

	imageIDs := make([]string, 0, len(result.Images))
	for i, data := range result.Images {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		final := data
		if req.MaskAssetID != "" {
			maskBytes, maskErr := m.assetBytes(req.MaskAssetID)
			if maskErr != nil {
				return maskErr
			}
			composited, compErr := MaskComposite(data, originalBytes, maskBytes, req.Params.MaskMode)
			if compErr != nil {
				return compErr
			}
			final = composited
		}

		imageID, saveErr := m.saveGeneratedImage(record, req, final, i)
		if saveErr != nil {
			return saveErr
		}
		imageIDs = append(imageIDs, imageID)
	}

	record.Set("status", "completed")
	record.Set("image_count", len(imageIDs))
	record.Set("error_code", 0)
	record.Set("error_message", "")
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("mark job completed: %w", err)
	}
	m.emitRecord(record)

	return nil
}

// saveGeneratedImage stores a generated image as a generated_images
// record with a file attachment and a thumbnail.
func (m *Manager) saveGeneratedImage(job *core.Record, req Request, data []byte, index int) (string, error) {
	col, err := m.app.FindCollectionByNameOrId("generated_images")
	if err != nil {
		return "", fmt.Errorf("find generated_images collection: %w", err)
	}

	record := core.NewRecord(col)
	record.Set("job", job.Id)
	record.Set("bytes", len(data))
	record.Set("seed", req.Params.Seed)
	record.Set("style_tag", req.Params.StyleType)
	record.Set("prompt_snapshot", req.Prompt)

	// Determine the image dimensions for the metadata.
	width, height := imageDimensions(data)
	record.Set("width", width)
	record.Set("height", height)

	name := fmt.Sprintf("image-%d.png", index+1)
	record.Set("file_name", name)

	file, err := filesystem.NewFileFromBytes(data, name)
	if err != nil {
		return "", fmt.Errorf("create image file: %w", err)
	}
	record.Set("file", file)

	// Attach a thumbnail so the gallery can render cheaply.
	if thumb, thumbErr := Thumbnail(data, 320); thumbErr == nil {
		thumbFile, thumbFileErr := filesystem.NewFileFromBytes(thumb, "thumbnail.png")
		if thumbFileErr == nil {
			record.Set("thumbnail", thumbFile)
		}
	}

	if err := m.app.Save(record); err != nil {
		return "", fmt.Errorf("save generated image: %w", err)
	}

	return record.Id, nil
}

// fail marks a job as failed with a human readable message.
func (m *Manager) fail(jobID string, cause error) {
	record, err := m.app.FindRecordById("image_jobs", jobID)
	if err != nil {
		return
	}

	// A cancelled job keeps its cancelled status.
	if record.GetString("status") == "cancelled" {
		return
	}

	code := 0
	message := cause.Error()

	var apiErr *minimax.Error
	if ok := asMiniMaxError(cause, &apiErr); ok {
		code = apiErr.StatusCode
		message = apiErr.HumanMessage()
	}

	record.Set("status", "failed")
	record.Set("error_code", code)
	record.Set("error_message", message)
	if err := m.app.Save(record); err != nil {
		return
	}
	m.emitRecord(record)
}

// asMiniMaxError unwraps a MiniMax API error from an error chain.
func asMiniMaxError(err error, target **minimax.Error) bool {
	for err != nil {
		if apiErr, ok := err.(*minimax.Error); ok {
			*target = apiErr
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// emitRecord publishes a job update.
func (m *Manager) emitRecord(record *core.Record) {
	emitter := m.currentEmitter()
	if emitter == nil {
		return
	}
	job, err := m.toJob(record)
	if err != nil {
		return
	}
	emitter.EmitJobUpdated(job)
}

// assetBytes reads the file bytes of an uploaded asset.
func (m *Manager) assetBytes(assetID string) ([]byte, error) {
	if assetID == "" {
		return nil, nil
	}
	record, err := m.app.FindRecordById("uploaded_assets", assetID)
	if err != nil {
		return nil, fmt.Errorf("find asset %q: %w", assetID, err)
	}
	return readFileBytes(record)
}

// assetDataURL renders an uploaded asset as a base64 data URL, which is
// the only way a desktop client can pass a reference image to MiniMax.
func (m *Manager) assetDataURL(assetID string) (string, error) {
	if assetID == "" {
		return "", nil
	}
	data, err := m.assetBytes(assetID)
	if err != nil {
		return "", err
	}
	mime := "image/png"
	if record, err := m.app.FindRecordById("uploaded_assets", assetID); err == nil {
		if stored := record.GetString("mime"); stored != "" {
			mime = stored
		}
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// toJob converts a PocketBase record into the frontend-facing job.
func (m *Manager) toJob(record *core.Record) (Job, error) {
	params := Params{}
	if raw := record.Get("params"); raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(encoded, &params)
		}
	}

	job := Job{
		ID:             record.Id,
		Prompt:         record.GetString("prompt"),
		NegativePrompt: record.GetString("negative_prompt"),
		Params:         params,
		Status:         record.GetString("status"),
		MiniMaxTaskID:  record.GetString("minimax_task_id"),
		ErrorMessage:   record.GetString("error_message"),
		ImageIDs:       []string{},
		Created:        record.GetString("created"),
		Updated:        record.GetString("updated"),
	}

	if code := record.GetInt("error_code"); code != 0 {
		job.ErrorCode = code
	}
	if seed := record.GetInt("seed_effective"); seed != 0 {
		job.SeedEffective = int64(seed)
	}

	// Collect the generated images for this job.
	records, err := m.app.FindRecordsByFilter(
		"generated_images",
		"job = {:job}",
		"-created",
		50,
		0,
		dbx.Params{"job": record.Id},
	)
	if err == nil {
		for _, imageRecord := range records {
			job.ImageIDs = append(job.ImageIDs, imageRecord.Id)
		}
	}

	return job, nil
}
