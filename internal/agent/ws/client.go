package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"ops/internal/protocol"
)

type Client struct {
	wsURL     string
	handshake protocol.AgentHandshake
	handler   func(protocol.DeployInstruction)

	mu     sync.RWMutex
	conn   *websocket.Conn
	sendMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

func NewClient(wsURL string, handshake protocol.AgentHandshake) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		wsURL:     wsURL,
		handshake: handshake,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (c *Client) SetInstructionHandler(handler func(protocol.DeployInstruction)) {
	c.handler = handler
}

func (c *Client) Start() {
	go c.loop()
}

func (c *Client) Stop() {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Client) SendReport(report protocol.TaskReport) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return nil
	}
	return conn.WriteJSON(report)
}

func (c *Client) loop() {
	backoff := 3 * time.Second
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(c.ctx, c.wsURL, nil)
		if err != nil {
			log.Printf("dial websocket: %v", err)
			time.Sleep(backoff)
			if backoff < 60*time.Second {
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
			continue
		}
		backoff = 3 * time.Second

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		if err := conn.WriteJSON(c.handshake); err != nil {
			_ = conn.Close()
			continue
		}

		if err := c.readLoop(conn); err != nil {
			log.Printf("websocket read loop ended: %v", err)
		}
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		_ = conn.Close()
	}
}

func (c *Client) readLoop(conn *websocket.Conn) error {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var envelope protocol.Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue
		}
		switch envelope.Type {
		case protocol.MessageTypeDeploy:
			var inst protocol.DeployInstruction
			if err := json.Unmarshal(data, &inst); err != nil {
				continue
			}
			if c.handler != nil {
				go c.handler(inst)
			}
		case protocol.MessageTypePong:
		}
	}
}
