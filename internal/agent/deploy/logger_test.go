package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployLoggerWritesExpectedFormat(t *testing.T) {
	t.Parallel()

	logDir := t.TempDir()
	logger, err := NewDeployLogger(logDir, "task-a")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	logger.Log("device-b", "stop", "output-1")
	logger.Log("device-c", "start", "output-2")
	if err := logger.Close(); err != nil {
		t.Fatalf("close logger: %v", err)
	}

	files, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(files))
	}
	content, err := os.ReadFile(filepath.Join(logDir, files[0].Name()))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[device-b] [stop] output-1") || !strings.Contains(text, "[device-c] [start] output-2") {
		t.Fatalf("unexpected log content: %s", text)
	}
}

func TestDeployLoggerDifferentTaskIDsProduceDifferentFiles(t *testing.T) {
	t.Parallel()

	logDir := t.TempDir()
	loggerA, err := NewDeployLogger(logDir, "task-a")
	if err != nil {
		t.Fatalf("logger a: %v", err)
	}
	loggerB, err := NewDeployLogger(logDir, "task-b")
	if err != nil {
		t.Fatalf("logger b: %v", err)
	}
	loggerA.Log("device-a", "step", "a")
	loggerB.Log("device-b", "step", "b")
	_ = loggerA.Close()
	_ = loggerB.Close()

	files, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 log files, got %d", len(files))
	}
}
