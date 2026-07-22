package logfire_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
)

func TestEmbeddedSchemaContent(t *testing.T) {
	schema := logfire.SchemaMetadata()
	if len(schema) == 0 {
		t.Fatal("expected non-empty embedded schema markdown")
	}

	// Compare with refs/schema.md if available relative to test location
	refsPath := filepath.Join("..", "..", "refs", "schema.md")
	refsContent, err := os.ReadFile(refsPath)
	if err == nil {
		if string(refsContent) != schema {
			t.Errorf("embedded schema does not match refs/schema.md content")
		}
	}
}
