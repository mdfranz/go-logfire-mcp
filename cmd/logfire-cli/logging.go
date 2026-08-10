package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func configureLogger() (*slog.Logger, func(), error) {
	logFileTarget := strings.TrimSpace(os.Getenv("LOGFIRE_CLI_LOGFILE"))
	if logFileTarget == "" {
		logFileTarget = "logfire-cli.log"
	}
	debugLogging := debugEnabled(os.Getenv("LOGFIRE_CLI_DEBUG"))
	return newLogger(logFileTarget, debugLogging)
}

func debugEnabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "off":
		return false
	default:
		return true
	}
}

func newLogger(target string, debug bool) (*slog.Logger, func(), error) {
	var writer io.Writer
	closeLog := func() {}

	switch strings.ToLower(target) {
	case "off":
		writer = io.Discard
	case "stderr":
		writer = os.Stderr
	default:
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("open logfile: %w", err)
		}
		writer = file
		closeLog = func() { _ = file.Close() }
	}

	level := slog.LevelDebug
	if !debug {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})), closeLog, nil
}
