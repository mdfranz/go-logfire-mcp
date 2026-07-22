package main

import (
	"fmt"
	"io"
)

// writeResult outputs the raw query result body to stdout.
func writeResult(w io.Writer, content string) error {
	_, err := io.WriteString(w, content)
	return err
}

// writeError outputs formatted error messages to stderr.
func writeError(w io.Writer, err error) {
	fmt.Fprintf(w, "Error: %v\n", err)
}
