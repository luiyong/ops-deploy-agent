package main

import (
	"log"
	"os"

	agentcfg "ops/internal/agent/config"
	agentdeploy "ops/internal/agent/deploy"
	agentws "ops/internal/agent/ws"
	"ops/internal/protocol"
)

func main() {
	configPath := "config/agent.yaml"
	if len(os.Args) > 2 && os.Args[1] == "--config" {
		configPath = os.Args[2]
	}

	cfg, err := agentcfg.Load(configPath)
	if err != nil {
		log.Fatalf("load agent config: %v", err)
	}

	handshake := protocol.AgentHandshake{
		Type:    protocol.MessageTypePing,
		AgentID: cfg.Agent.ID,
		Devices: make([]protocol.DeviceInfo, 0, len(cfg.Devices)),
	}
	for _, device := range cfg.Devices {
		handshake.Devices = append(handshake.Devices, protocol.DeviceInfo{ID: device.ID})
	}

	client := agentws.NewClient(cfg.Server.WSURL, handshake)
	runner := agentdeploy.NewRunner(cfg)
	client.SetInstructionHandler(func(inst protocol.DeployInstruction) {
		runner.Run(inst, client)
	})
	client.Start()

	select {}
}
