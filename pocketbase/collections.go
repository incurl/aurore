// Package pocketbase embeds PocketBase as a Go library and owns the
// Aurore persistence schema.
//
// PocketBase is never run as a standalone process. The app bootstraps the
// library, runs Go migrations and mounts PocketBase's own REST/file API
// onto the Wails asset server so that generated images are served from
// the same origin as the UI.
//
// The schema itself is defined by the Go migrations in the migrations
// subpackage, which PocketBase tracks and applies. This file only holds
// the names and enumerations shared with the rest of the application.
package pocketbase

// Collection names used by Aurore.
const (
	// CollectionImageJobs stores generation job metadata.
	CollectionImageJobs = "image_jobs"
	// CollectionGeneratedImages stores the final generated artwork.
	CollectionGeneratedImages = "generated_images"
	// CollectionPromptPresets stores saved prompt templates.
	CollectionPromptPresets = "prompt_presets"
	// CollectionAppSettings stores configuration and encrypted secrets.
	CollectionAppSettings = "app_settings"
	// CollectionUploadedAssets stores user-uploaded reference images and
	// inpaint masks.
	CollectionUploadedAssets = "uploaded_assets"
)

// Job status values stored in image_jobs.status.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// AllJobStatuses lists every valid job status.
func AllJobStatuses() []string {
	return []string{StatusPending, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled}
}

// Uploaded asset kinds stored in uploaded_assets.kind.
const (
	AssetKindReference = "reference"
	AssetKindMask      = "mask"
)
