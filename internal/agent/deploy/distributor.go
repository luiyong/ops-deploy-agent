package deploy

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	agentcfg "ops/internal/agent/config"
)

type Distributor struct{}

func (d Distributor) Distribute(localJarPath string, device agentcfg.DeviceConfig, logger *DeployLogger) error {
	target := fmt.Sprintf("%s@%s:%s/", device.SSHUser, device.Host, device.TempDir)
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-P", fmt.Sprintf("%d", device.SSHPort),
		localJarPath,
		target,
	}
	cmd := exec.Command("scp", args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	logger.Log(device.ID, "scp", output.String())
	if err != nil {
		return fmt.Errorf("scp %s to %s failed: %w", filepath.Base(localJarPath), device.ID, err)
	}
	return nil
}
