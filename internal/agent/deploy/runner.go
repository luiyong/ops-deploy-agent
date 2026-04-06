package deploy

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"ops/internal/protocol"

	agentcfg "ops/internal/agent/config"
)

type Reporter interface {
	SendReport(protocol.TaskReport) error
}

type Runner struct {
	cfg         agentcfg.Config
	downloader  Downloader
	distributor Distributor
	replacer    Replacer
	mu          sync.Mutex
}

func NewRunner(cfg agentcfg.Config) *Runner {
	return &Runner{
		cfg:         cfg,
		downloader:  Downloader{},
		distributor: Distributor{},
		replacer:    Replacer{},
	}
}

func (r *Runner) Run(inst protocol.DeployInstruction, reporter Reporter) {
	go r.run(inst, reporter)
}

func (r *Runner) run(inst protocol.DeployInstruction, reporter Reporter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now().UTC()
	logger, err := NewDeployLogger(r.cfg.Agent.LogDir, inst.TaskID)
	if err != nil {
		_ = reporter.SendReport(protocol.TaskReport{
			Type:            protocol.MessageTypeReport,
			TaskID:          inst.TaskID,
			ServiceName:     inst.ServiceName,
			JarName:         inst.JarName,
			TargetDeviceIDs: inst.TargetDeviceIDs,
			StartTime:       start,
			EndTime:         time.Now().UTC(),
			DeviceResults: []protocol.DeviceResult{{
				DeviceID: "agent",
				Status:   protocol.DeviceStatusFailed,
				ErrorMsg: err.Error(),
			}},
		})
		return
	}
	defer logger.Close()

	logger.Log("agent", "start", "received deploy instruction")
	localJarPath, err := r.downloader.Download(inst.JarDownloadURL, r.cfg.Agent.Workspace)
	if err != nil {
		logger.Log("agent", "download", err.Error())
		_ = reporter.SendReport(protocol.TaskReport{
			Type:            protocol.MessageTypeReport,
			TaskID:          inst.TaskID,
			ServiceName:     inst.ServiceName,
			JarName:         inst.JarName,
			TargetDeviceIDs: inst.TargetDeviceIDs,
			StartTime:       start,
			EndTime:         time.Now().UTC(),
			DeviceResults: []protocol.DeviceResult{{
				DeviceID: "agent",
				Status:   protocol.DeviceStatusFailed,
				ErrorMsg: err.Error(),
			}},
		})
		return
	}

	var results []protocol.DeviceResult
	uploadedJarFilename := filepath.Base(localJarPath)
	for _, deviceID := range inst.TargetDeviceIDs {
		device, service, resolveErr := r.resolve(deviceID, inst.ServiceName)
		if resolveErr != nil {
			logger.Log(deviceID, "resolve", resolveErr.Error())
			results = append(results, protocol.DeviceResult{
				DeviceID: deviceID,
				Status:   protocol.DeviceStatusFailed,
				ErrorMsg: resolveErr.Error(),
			})
			continue
		}

		if err := r.distributor.Distribute(localJarPath, device, logger); err != nil {
			logger.Log(deviceID, "scp", err.Error())
			results = append(results, protocol.DeviceResult{
				DeviceID: deviceID,
				Status:   protocol.DeviceStatusFailed,
				ErrorMsg: err.Error(),
			})
			continue
		}

		if err := r.replacer.Replace(device, service, uploadedJarFilename, logger); err != nil {
			logger.Log(deviceID, "replace", err.Error())
			results = append(results, protocol.DeviceResult{
				DeviceID: deviceID,
				Status:   protocol.DeviceStatusFailed,
				ErrorMsg: err.Error(),
			})
			continue
		}

		results = append(results, protocol.DeviceResult{
			DeviceID: deviceID,
			Status:   protocol.DeviceStatusSuccess,
		})
	}

	logger.Log("agent", "summary", fmt.Sprintf("finished with %d device results", len(results)))
	_ = reporter.SendReport(protocol.TaskReport{
		Type:            protocol.MessageTypeReport,
		TaskID:          inst.TaskID,
		ServiceName:     inst.ServiceName,
		JarName:         inst.JarName,
		TargetDeviceIDs: inst.TargetDeviceIDs,
		StartTime:       start,
		EndTime:         time.Now().UTC(),
		DeviceResults:   results,
	})
}

func (r *Runner) resolve(deviceID, serviceName string) (agentcfg.DeviceConfig, agentcfg.ServiceConfig, error) {
	var device agentcfg.DeviceConfig
	var foundDevice bool
	for _, candidate := range r.cfg.Devices {
		if candidate.ID == deviceID {
			device = candidate
			foundDevice = true
			break
		}
	}
	if !foundDevice {
		return agentcfg.DeviceConfig{}, agentcfg.ServiceConfig{}, fmt.Errorf("device %q not configured", deviceID)
	}

	for _, service := range r.cfg.Services {
		if service.DeviceID == deviceID && service.ServiceName == serviceName {
			return device, service, nil
		}
	}
	return agentcfg.DeviceConfig{}, agentcfg.ServiceConfig{}, fmt.Errorf("service %q not configured for device %q", serviceName, deviceID)
}
