package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that records the real image format.
func init() {
	registerMigration("1731000000_image_mime.go", func(app core.App) error {
		return addImageMimeField(app)
	})
}

// addImageMimeField adds the "mime" field to the generated_images
// collection.
//
// The format of a generated image is now detected from its bytes, so
// that the stored file is not a JPEG with a PNG name. The field is
// added by a migration of its own, because the initial migration has
// already run on every existing database.
//
// Existing rows keep an empty mime, which the readers treat as "the
// format was not recorded"; those images were stored with a .png name
// whatever they really were.
func addImageMimeField(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("generated_images")
	if err != nil {
		return err
	}

	// A field that is already there means the migration already ran.
	if collection.Fields.GetByName("mime") != nil {
		return nil
	}

	collection.Fields.Add(&core.TextField{Name: "mime", Max: 100})

	return app.Save(collection)
}
