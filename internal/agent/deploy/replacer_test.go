package deploy

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	agentcfg "ops/internal/agent/config"
)

func TestReplacerScenarios(t *testing.T) {
	device := agentcfg.DeviceConfig{
		ID:      "device-b",
		Host:    "127.0.0.1",
		SSHUser: "deploy",
		SSHPort: 22,
		TempDir: "/tmp/deploy",
	}
	service := agentcfg.ServiceConfig{
		DeviceID:      "device-b",
		ServiceName:   "svc-a",
		DeployDir:     "/srv/app",
		TargetJarName: "app.jar",
		StartScript:   "/srv/app/bin/start.sh",
		StopScript:    "/srv/app/bin/stop.sh",
		ProcessName:   "svc-a",
	}

	t.Run("all steps succeed", func(t *testing.T) {
		logger := newMemoryLogger(t)
		var cmds []string
		restore := replaceSSHExecutor(func(_ agentcfg.DeviceConfig, remoteCmd string) (string, error) {
			cmds = append(cmds, remoteCmd)
			if strings.Contains(remoteCmd, "pgrep -f") {
				if len(cmds) < 3 {
					return "", nil
				}
				return "123\n", nil
			}
			return "ok", nil
		})
		defer restore()

		if err := (Replacer{}).Replace(device, service, "svc-a-1.2.jar", logger); err != nil {
			t.Fatalf("replace: %v", err)
		}
		if !containsCommand(cmds, "mv '/tmp/deploy/svc-a-1.2.jar' '/srv/app/app.jar'") {
			t.Fatalf("expected mv command with target_jar_name, got %v", cmds)
		}
	})

	t.Run("stop timeout prevents later steps", func(t *testing.T) {
		logger := newMemoryLogger(t)
		var mu sync.Mutex
		var cmds []string
		restore := replaceSSHExecutor(func(_ agentcfg.DeviceConfig, remoteCmd string) (string, error) {
			mu.Lock()
			cmds = append(cmds, remoteCmd)
			mu.Unlock()
			if strings.Contains(remoteCmd, "pgrep -f") {
				return "123\n", nil
			}
			return "ok", nil
		})
		defer restore()
		restoreTiming := replaceWaitTiming(1*time.Millisecond, 5*time.Millisecond)
		defer restoreTiming()

		err := (Replacer{}).Replace(device, service, "svc-a-1.2.jar", logger)
		if err == nil || !strings.Contains(err.Error(), "停止超时") {
			t.Fatalf("expected stop timeout, got %v", err)
		}
		if containsCommand(cmds, service.StartScript) || containsCommand(cmds, "mv '/tmp/deploy/svc-a-1.2.jar' '/srv/app/app.jar'") {
			t.Fatalf("unexpected later commands after stop timeout: %v", cmds)
		}
	})

	t.Run("start timeout returns error", func(t *testing.T) {
		logger := newMemoryLogger(t)
		restore := replaceSSHExecutor(func(_ agentcfg.DeviceConfig, remoteCmd string) (string, error) {
			switch {
			case remoteCmd == service.StopScript:
				return "stopped", nil
			case strings.Contains(remoteCmd, "pgrep -f"):
				if strings.Contains(remoteCmd, shellQuote(service.ProcessName)) {
					return "", nil
				}
			case strings.HasPrefix(remoteCmd, "mv "):
				return "moved", nil
			case remoteCmd == service.StartScript:
				return "started", nil
			}
			return "", nil
		})
		defer restore()
		restoreTiming := replaceWaitTiming(1*time.Millisecond, 5*time.Millisecond)
		defer restoreTiming()

		err := (Replacer{}).Replace(device, service, "svc-a-1.2.jar", logger)
		if err == nil || !strings.Contains(err.Error(), "启动超时") {
			t.Fatalf("expected start timeout, got %v", err)
		}
	})

	t.Run("stop command failure aborts flow", func(t *testing.T) {
		logger := newMemoryLogger(t)
		var cmds []string
		restore := replaceSSHExecutor(func(_ agentcfg.DeviceConfig, remoteCmd string) (string, error) {
			cmds = append(cmds, remoteCmd)
			if remoteCmd == service.StopScript {
				return "stop failed", fmt.Errorf("exit status 1")
			}
			return "", nil
		})
		defer restore()

		err := (Replacer{}).Replace(device, service, "svc-a-1.2.jar", logger)
		if err == nil || !strings.Contains(err.Error(), "stop step failed") {
			t.Fatalf("expected stop step failure, got %v", err)
		}
		if len(cmds) != 1 {
			t.Fatalf("expected only stop command to run, got %v", cmds)
		}
	})
}

func replaceSSHExecutor(fn func(agentcfg.DeviceConfig, string) (string, error)) func() {
	original := sshExecutor
	sshExecutor = fn
	return func() { sshExecutor = original }
}

func replaceWaitTiming(interval, timeout time.Duration) func() {
	originalInterval := processPollInterval
	originalTimeout := processWaitTimeout
	processPollInterval = interval
	processWaitTimeout = timeout
	return func() {
		processPollInterval = originalInterval
		processWaitTimeout = originalTimeout
	}
}

func containsCommand(commands []string, needle string) bool {
	for _, cmd := range commands {
		if cmd == needle {
			return true
		}
	}
	return false
}

func newMemoryLogger(t *testing.T) *DeployLogger {
	t.Helper()
	logger, err := NewDeployLogger(t.TempDir(), "task-1")
	if err != nil {
		t.Fatalf("new deploy logger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}
