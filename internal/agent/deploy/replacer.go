package deploy

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	agentcfg "ops/internal/agent/config"
)

type Replacer struct{}

var (
	sshExecutor         = defaultSSHExecutor
	processPollInterval = 3 * time.Second
	processWaitTimeout  = 60 * time.Second
)

func (r Replacer) Replace(device agentcfg.DeviceConfig, service agentcfg.ServiceConfig, uploadedJarFilename string, logger *DeployLogger) error {
	steps := []struct {
		name string
		cmd  string
	}{
		{name: "stop", cmd: service.StopScript},
	}

	for _, step := range steps {
		if _, err := runSSH(device, step.cmd, logger, step.name); err != nil {
			return fmt.Errorf("%s step failed: %w", step.name, err)
		}
	}

	if err := waitForProcess(device, service.ProcessName, false, logger, "wait-stop"); err != nil {
		return err
	}

	moveCmd := fmt.Sprintf(
		"mv %s %s",
		shellQuote(strings.TrimRight(serviceTempPath(device.TempDir), "/")+"/"+uploadedJarFilename),
		shellQuote(strings.TrimRight(service.DeployDir, "/")+"/"+service.TargetJarName),
	)
	if _, err := runSSH(device, moveCmd, logger, "move-jar"); err != nil {
		return fmt.Errorf("move jar failed: %w", err)
	}

	if _, err := runSSH(device, service.StartScript, logger, "start"); err != nil {
		return fmt.Errorf("start step failed: %w", err)
	}

	if err := waitForProcess(device, service.ProcessName, true, logger, "wait-start"); err != nil {
		return err
	}

	return nil
}

func waitForProcess(device agentcfg.DeviceConfig, processName string, shouldExist bool, logger *DeployLogger, step string) error {
	timeout := time.After(processWaitTimeout)
	ticker := time.NewTicker(processPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			if shouldExist {
				return fmt.Errorf("启动超时")
			}
			return fmt.Errorf("停止超时")
		case <-ticker.C:
			output, err := runSSH(device, "pgrep -f "+shellQuote(processName), logger, step)
			exists := strings.TrimSpace(output) != ""
			if shouldExist && exists {
				return nil
			}
			if !shouldExist && !exists {
				return nil
			}
			if err != nil && shouldExist {
				continue
			}
		}
	}
}

func runSSH(device agentcfg.DeviceConfig, remoteCmd string, logger *DeployLogger, step string) (string, error) {
	output, err := sshExecutor(device, remoteCmd)
	logger.Log(device.ID, step, output)
	if err != nil {
		return output, fmt.Errorf("ssh command failed: %w", err)
	}
	return output, nil
}

func defaultSSHExecutor(device agentcfg.DeviceConfig, remoteCmd string) (string, error) {
	target := fmt.Sprintf("%s@%s", device.SSHUser, device.Host)
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-p", fmt.Sprintf("%d", device.SSHPort),
		target,
		remoteCmd,
	}
	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func serviceTempPath(tempDir string) string {
	return strings.TrimRight(tempDir, "/")
}
