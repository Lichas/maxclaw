package webui

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/gorilla/websocket"
)

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Client represents a WebSocket client
type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from Electron app and local development
		return true
	},
}

// WebSocketMessageType 消息类型
type WebSocketMessageType string

const (
	WSMessageTypeChat      WebSocketMessageType = "chat"
	WSMessageTypeInterrupt WebSocketMessageType = "interrupt"
	WSMessageTypeStream    WebSocketMessageType = "stream"
	WSMessageTypeStatus    WebSocketMessageType = "status"
)

// WSMessage WebSocket 消息结构
type WSMessage struct {
	Type      WebSocketMessageType `json:"type"`
	Session   string               `json:"session,omitempty"`
	Content   string               `json:"content,omitempty"`
	Mode      string               `json:"mode,omitempty"` // "cancel" | "append"
	Timestamp int64                `json:"timestamp,omitempty"`
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run starts the WebSocket hub event loop
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(messageType string, payload interface{}) {
	data, _ := json.Marshal(map[string]interface{}{
		"type":    messageType,
		"payload": payload,
	})
	h.broadcast <- data
}

// ServerReference 用于访问 Server 方法（在 handleWebSocket 中设置）
type ServerReference struct {
	server *Server
}

var serverRef = &ServerReference{}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 处理客户端消息
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("WebSocket message parse error: %v", err)
			continue
		}

		switch msg.Type {
		case WSMessageTypeChat:
			// 普通聊天消息 - 通过 Bus 发送
			if serverRef.server != nil {
				inbound := bus.NewInboundMessage("desktop", "user", msg.Session, msg.Content)
				serverRef.server.agentLoop.Bus.PublishInbound(inbound)
			}

		case WSMessageTypeInterrupt:
			// 中断请求
			if serverRef.server != nil {
				inbound := bus.NewInboundMessage("desktop", "user", msg.Session, msg.Content)
				mode := agent.InterruptCancel
				if msg.Mode == "append" {
					mode = agent.InterruptAppend
				}

				// 设置中断模式到消息的元数据中
				_ = mode
				serverRef.server.agentLoop.HandleInterruption(inbound)
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

// handleWebSocket upgrades HTTP connection to WebSocket
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// 设置 server 引用以便 readPump 访问
	serverRef.server = s

	client := &Client{
		hub:  s.wsHub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
