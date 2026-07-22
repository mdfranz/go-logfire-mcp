package logfire

import _ "embed"

//go:embed schema.md
var EmbeddedSchemaMarkdown string

// SchemaMetadata returns the schema markdown document.
func SchemaMetadata() string {
	return EmbeddedSchemaMarkdown
}
