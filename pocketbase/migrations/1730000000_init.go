// Package migrations contains the Go migrations that create and evolve
// the Aurore PocketBase schema.
//
// Migrations are registered with PocketBase's own migration list and are
// applied by Service.Bootstrap, so PocketBase's own migration tooling
// and bookkeeping are used rather than a custom mechanism.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// registerMigration is a small alias so the migration bodies stay
// readable.
func registerMigration(file string, up func(core.App) error) {
	core.AppMigrations.Register(up, func(core.App) error { return nil }, file)
}

// init registers the Aurore schema migration.
func init() {
	registerMigration("1730000000_init.go", func(app core.App) error {
		return createAuroreSchema(app)
	})
}

// createAuroreSchema creates the five Aurore collections.
//
// The collections are created in dependency order so that relation
// fields can reference an existing collection id.
func createAuroreSchema(app core.App) error {
	// --- image_jobs -------------------------------------------------
	imageJobs := core.NewBaseCollection("image_jobs")
	imageJobs.Fields.Add(
		&core.TextField{Name: "prompt", Required: true},
		&core.TextField{Name: "negative_prompt"},
		&core.TextField{Name: "minimax_task_id", Max: 100},
		&core.SelectField{
			Name:      "status",
			Values:    []string{"pending", "running", "completed", "failed", "cancelled"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "error_code", Max: 20},
		&core.TextField{Name: "error_message"},
		&core.JSONField{Name: "params"},
		&core.NumberField{Name: "seed_requested"},
		&core.NumberField{Name: "seed_effective"},
		&core.NumberField{Name: "image_count"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	// The frontend never writes directly: all mutations go through the
	// Go services, so only the read rules are opened.
	imageJobs.ListRule = types.Pointer("")
	imageJobs.ViewRule = types.Pointer("")
	if err := app.Save(imageJobs); err != nil {
		return err
	}

	// --- generated_images -------------------------------------------
	generatedImages := core.NewBaseCollection("generated_images")
	generatedImages.Fields.Add(
		&core.RelationField{
			Name:         "job",
			CollectionId: imageJobs.Id,
			MaxSelect:    1,
		},
		&core.FileField{Name: "file", MaxSelect: 1, MaxSize: 25 << 20},
		&core.FileField{Name: "thumbnail", MaxSelect: 1, MaxSize: 5 << 20},
		&core.NumberField{Name: "width"},
		&core.NumberField{Name: "height"},
		&core.NumberField{Name: "bytes"},
		&core.NumberField{Name: "seed"},
		&core.TextField{Name: "style_tag", Max: 100},
		&core.TextField{Name: "prompt_snapshot"},
		&core.TextField{Name: "file_name", Max: 200},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	generatedImages.ListRule = types.Pointer("")
	generatedImages.ViewRule = types.Pointer("")
	if err := app.Save(generatedImages); err != nil {
		return err
	}

	// --- prompt_presets ---------------------------------------------
	promptPresets := core.NewBaseCollection("prompt_presets")
	promptPresets.Fields.Add(
		&core.TextField{Name: "name", Required: true, Max: 200},
		&core.TextField{Name: "prompt"},
		&core.TextField{Name: "negative_prompt"},
		&core.JSONField{Name: "params"},
		&core.BoolField{Name: "is_default"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	promptPresets.ListRule = types.Pointer("")
	promptPresets.ViewRule = types.Pointer("")
	if err := app.Save(promptPresets); err != nil {
		return err
	}

	// --- app_settings -----------------------------------------------
	// Never exposed to the frontend: the API key is only ever read by
	// the Go backend.
	appSettings := core.NewBaseCollection("app_settings")
	appSettings.Fields.Add(
		&core.TextField{Name: "key", Required: true, Max: 100},
		&core.JSONField{Name: "value"},
		&core.BoolField{Name: "encrypted"},
		&core.AutodateField{Name: "updated", OnUpdate: true},
	)
	if err := app.Save(appSettings); err != nil {
		return err
	}

	// --- uploaded_assets --------------------------------------------
	// Inherits the same read rules as the other user-facing collections.
	uploadedAssets := core.NewBaseCollection("uploaded_assets")
	uploadedAssets.Fields.Add(
		&core.FileField{Name: "file", MaxSelect: 1, MaxSize: 12 << 20},
		&core.SelectField{
			Name:      "kind",
			Values:    []string{"reference", "mask"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "job", Max: 15},
		&core.TextField{Name: "mime", Max: 100},
		&core.NumberField{Name: "bytes"},
		&core.NumberField{Name: "width"},
		&core.NumberField{Name: "height"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	uploadedAssets.ListRule = types.Pointer("")
	uploadedAssets.ViewRule = types.Pointer("")
	if err := app.Save(uploadedAssets); err != nil {
		return err
	}

	return nil
}
