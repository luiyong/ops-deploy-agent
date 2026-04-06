package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DeployLogger struct {
	mu   sync.Mutex
	file *os.File
}

func NewDeployLogger(logDir, taskID string) (*DeployLogger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	filename := fmt.Sprintf("%s_%s.log", taskID, time.Now().UTC().Format("20060102T150405Z"))
	file, err := os.Create(filepath.Join(logDir, filename))
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	return &DeployLogger{file: file}, nil
}

func (l *DeployLogger) Log(deviceID, step, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	line := fmt.Sprintf("[%s] [%s] [%s] %s\n", time.Now().UTC().Format(time.RFC3339), deviceID, step, message)
	_, _ = l.file.WriteString(line)
}

func (l *DeployLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
