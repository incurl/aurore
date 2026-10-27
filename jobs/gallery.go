package jobs

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// ListJobs returns the most recent jobs, newest first.
func (m *Manager) ListJobs(limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	records, err := m.app.FindRecordsByFilter(
		"image_jobs",
		"",
		"-created",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	jobs := make([]Job, 0, len(records))
	for _, record := range records {
		job, convErr := m.toJob(record)
		if convErr != nil {
			return nil, convErr
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// GetJob returns a single job.
func (m *Manager) GetJob(jobID string) (Job, error) {
	record, err := m.app.FindRecordById("image_jobs", jobID)
	if err != nil {
		return Job{}, fmt.Errorf("find job %q: %w", jobID, err)
	}
	return m.toJob(record)
}

// ListImages returns the generated images, newest first. When jobID is
// not empty only the images of that job are returned.
func (m *Manager) ListImages(jobID string, limit int) ([]Image, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	filter := ""
	var params []dbx.Params
	if jobID != "" {
		filter = "job = {:job}"
		params = append(params, dbx.Params{"job": jobID})
	}

	records, err := m.app.FindRecordsByFilter(
		"generated_images",
		filter,
		"-created",
		limit,
		0,
		params...,
	)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	images := make([]Image, 0, len(records))
	for _, record := range records {
		images = append(images, m.toImage(record))
	}
	return images, nil
}

// GetImage returns a single generated image.
func (m *Manager) GetImage(imageID string) (Image, error) {
	record, err := m.app.FindRecordById("generated_images", imageID)
	if err != nil {
		return Image{}, fmt.Errorf("find image %q: %w", imageID, err)
	}
	return m.toImage(record), nil
}

// toImage converts a PocketBase record into the frontend-facing image.
//
// The URLs point at PocketBase's native file route, which is mounted on
// the same origin as the UI.
func (m *Manager) toImage(record *core.Record) Image {
	image := Image{
		ID:       record.Id,
		JobID:    record.GetString("job"),
		Width:    record.GetInt("width"),
		Height:   record.GetInt("height"),
		Bytes:    record.GetInt("bytes"),
		Seed:     int64(record.GetInt("seed")),
		StyleTag: record.GetString("style_tag"),
		Prompt:   record.GetString("prompt_snapshot"),
		Created:  record.GetString("created"),
	}

	// The file field holds the stored filename, which includes the
	// random suffix PocketBase appends. It is the exact name the file
	// route expects.
	if name := record.GetString("file"); name != "" {
		image.URL = fileURL(record.Id, name)
	}
	if thumb := record.GetString("thumbnail"); thumb != "" {
		image.ThumbnailURL = fileURL(record.Id, thumb)
	}
	return image
}

// fileURL builds the native PocketBase file URL for a record attachment.
func fileURL(recordID, filename string) string {
	return "/api/files/generated_images/" + recordID + "/" + filename
}

// DeleteImage removes a generated image and its files.
func (m *Manager) DeleteImage(imageID string) error {
	record, err := m.app.FindRecordById("generated_images", imageID)
	if err != nil {
		return fmt.Errorf("find image %q: %w", imageID, err)
	}
	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("delete image %q: %w", imageID, err)
	}
	return nil
}

// ExportImageBytes returns the raw bytes of a generated image so the
// frontend can save it to disk.
func (m *Manager) ExportImageBytes(imageID string) ([]byte, error) {
	record, err := m.app.FindRecordById("generated_images", imageID)
	if err != nil {
		return nil, fmt.Errorf("find image %q: %w", imageID, err)
	}
	return readFileBytes(m.app, record)
}
