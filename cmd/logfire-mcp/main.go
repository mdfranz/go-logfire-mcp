package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/mdfranz/go-logfire-mcp/internal/logfire"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logFileTarget := strings.TrimSpace(os.Getenv("LOGFIRE_MCP_LOGFILE"))
	if logFileTarget == "" {
		logFileTarget = "logfire-mcp.log"
	}
	debugLogging := debugEnabled(os.Getenv("LOGFIRE_MCP_DEBUG"))
	logger, closeLog, err := newLogger(logFileTarget, debugLogging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure logging: %v\n", err)
		return 1
	}
	defer closeLog()
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := logfire.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		return 1
	}

	// Read result byte cap for MCP tool
	maxResultBytes := int64(1 * 1024 * 1024) // 1 MiB default for MCP
	if capStr := os.Getenv("LOGFIRE_MCP_MAX_RESULT_BYTES"); capStr != "" {
		if parsedCap, parseErr := strconv.ParseInt(capStr, 10, 64); parseErr == nil && parsedCap > 0 {
			maxResultBytes = parsedCap
		}
	}

	// PID Lockfile handling
	lockFilePath := strings.TrimSpace(os.Getenv("LOGFIRE_MCP_LOCKFILE"))
	if lockFilePath != "" && strings.ToLower(lockFilePath) != "off" {
		unlock, lockErr := acquireLock(lockFilePath)
		if lockErr != nil {
			slog.Error("failed to acquire lockfile", "path", lockFilePath, "error", lockErr)
			return 1
		}
		defer unlock()
	}

	slog.Info("starting logfire-mcp server",
		"version", version,
		"region", cfg.Region,
		"base_url", cfg.BaseURL,
		"max_result_bytes", maxResultBytes,
		"lockfile", lockFilePath,
		"logfile", logFileTarget,
		"debug", debugLogging,
	)

	client := logfire.NewClient(cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "logfire-mcp",
		Version: version,
	}, nil)

	registerTools(server, client, maxResultBytes)

	transport := &mcp.StdioTransport{}
	if err := server.Run(ctx, transport); err != nil {
		slog.Error("mcp server error", "error", err)
		return 1
	}

	slog.Info("logfire-mcp server stopped gracefully")
	return 0
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

func acquireLock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for lockfile: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lockfile: %w", err)
	}

	// Try non-blocking flock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another instance of logfire-mcp is already running (locked: %s)", path)
	}

	// Write PID to lockfile
	f.Truncate(0)
	f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())

	unlock := func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		os.Remove(path)
	}

	return unlock, nil
}
