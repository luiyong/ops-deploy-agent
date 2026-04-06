package deploy

import (
	"testing"

	"github.com/gorilla/websocket"

	"ops/internal/protocol"
)

func TestAgentHubRegisterAndUnregister(t *testing.T) {
	t.Parallel()

	hub := NewAgentHub()
	conn := &websocket.Conn{}
	handshake := protocol.AgentHandshake{
		AgentID: "agent-a",
		Devices: []protocol.DeviceInfo{{ID: "device-b"}, {ID: "device-c"}},
	}

	hub.Register(conn, handshake)
	if !hub.IsOnline() {
		t.Fatalf("expected hub online after register")
	}

	devices := hub.GetDevices()
	if len(devices) != 2 || devices[0].ID != "device-b" || devices[1].ID != "device-c" {
		t.Fatalf("unexpected devices: %+v", devices)
	}

	hub.Unregister(conn)
	if hub.IsOnline() {
		t.Fatalf("expected hub offline after unregister")
	}
}

func TestAgentHubSendInstructionOffline(t *testing.T) {
	t.Parallel()

	hub := NewAgentHub()
	err := hub.SendInstruction(protocol.DeployInstruction{TaskID: "task-1"})
	if err == nil {
		t.Fatalf("expected offline send to fail")
	}
}
