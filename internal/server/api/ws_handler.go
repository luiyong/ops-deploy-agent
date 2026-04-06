package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"ops/internal/protocol"
	"ops/internal/server/deploy"
)

type WSHandler struct {
	hub      *deploy.AgentHub
	upgrader websocket.Upgrader
}

func NewWSHandler(hub *deploy.AgentHub) *WSHandler {
	return &WSHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "upgrade websocket", err)
		return
	}

	_, data, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return
	}

	var handshake protocol.AgentHandshake
	if err := json.Unmarshal(data, &handshake); err != nil {
		_ = conn.Close()
		return
	}
	h.hub.Register(conn, handshake)
	defer func() {
		h.hub.Unregister(conn)
		_ = conn.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope protocol.Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			log.Printf("decode ws envelope: %v", err)
			continue
		}

		switch envelope.Type {
		case protocol.MessageTypeReport:
			var report protocol.TaskReport
			if err := json.Unmarshal(data, &report); err != nil {
				log.Printf("decode task report: %v", err)
				continue
			}
			h.hub.HandleReport(report)
		case protocol.MessageTypePing:
			_ = conn.WriteJSON(protocol.Envelope{Type: protocol.MessageTypePong})
		default:
			log.Printf("ignore unsupported ws message type: %s", envelope.Type)
		}
	}
}
