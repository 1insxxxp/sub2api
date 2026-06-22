package schema

import (
	"testing"

	"entgo.io/ent/entc/load"
	"github.com/stretchr/testify/require"
)

func TestUserImageSchema(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, schema := range spec.Schemas {
		schemas[schema.Name] = schema
	}

	userImage := requireSchema(t, schemas, "UserImage")
	requireSchemaFields(t, userImage,
		"user_id",
		"mode",
		"model",
		"prompt",
		"aspect_ratio",
		"size",
		"image_url",
		"storage_driver",
		"storage_object_key",
		"mime_type",
		"bytes",
		"cost",
		"usage_log_id",
		"source_image_count",
		"expires_at",
		"deleted_at",
		"created_at",
		"updated_at",
	)
	requireHasIndex(t, userImage, "user_id", "created_at")
	requireHasIndex(t, userImage, "deleted_at")
}

func requireHasIndex(t *testing.T, schema *load.Schema, fields ...string) {
	t.Helper()

	for _, index := range schema.Indexes {
		if len(index.Fields) != len(fields) {
			continue
		}
		match := true
		for i := range fields {
			if index.Fields[i] != fields[i] {
				match = false
				break
			}
		}
		if match {
			return
		}
	}

	require.Failf(t, "missing index", "schema %s should include index on %v", schema.Name, fields)
}
