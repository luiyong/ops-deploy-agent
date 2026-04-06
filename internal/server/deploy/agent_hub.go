package deploy

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"

	"ops/internal/protocol"
)

type AgentHub struct {
	mu           sync.RWMutex
	conn         *websocket.Conn
	agentID      string
	devices      []protocol.DeviceInfo
	reportHandle func(protocol.TaskReport)
}

func NewAgentHub() *AgentHub {
	return &AgentHub{}
}

func (h *AgentHub) Register(conn *websocket.Conn, handshake protocol.AgentHandshake) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conn = conn
	h.agentID = handshake.AgentID
	h.devices = handshake.Devices
}

func (h *AgentHub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn == conn {
		h.conn = nil
		h.agentID = ""
		h.devices = nil
	}
}

func (h *AgentHub) IsOnline() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conn != nil
}

func (h *AgentHub) GetDevices() []protocol.DeviceInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]protocol.DeviceInfo, len(h.devices))
	copy(out, h.devices)
	return out
}

func (h *AgentHub) SendInstruction(inst protocol.DeployInstruction) error {
	h.mu.RLock()
	conn := h.conn
	h.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("agent is offline")
	}
	return conn.WriteJSON(inst)
}

func (h *AgentHub) OnReport(handler func(protocol.TaskReport)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reportHandle = handler
}

func (h *AgentHub) HandleReport(report protocol.TaskReport) {
	h.mu.RLock()
	handler := h.reportHandle
	h.mu.RUnlock()
	if handler != nil {
		handler(report)
	}
}
