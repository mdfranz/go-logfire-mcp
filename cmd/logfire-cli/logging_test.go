package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureLoggerDefaultsToDebug(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "cli.log")
	t.Setenv("LOGFIRE_CLI_LOGFILE", logPath)
	t.Setenv("LOGFIRE_CLI_DEBUG", "")

	logger, closeLog, err := configureLogger()
	if err != nil {
		t.Fatalf("configureLogger() error: %v", err)
	}
	logger.Debug("debug message")
	closeLog()

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(contents), "debug message") {
		t.Fatalf("default logger did not emit debug message: %q", contents)
	}
}
