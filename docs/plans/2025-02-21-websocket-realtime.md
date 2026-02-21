# WebSocket Real-Time Push Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement WebSocket-based real-time message push to replace HTTP polling, reducing latency and server load for live message updates.

**Architecture:** Backend Gateway upgrades HTTP connections to WebSocket and maintains client connections. Messages are broadcast to connected clients immediately when available. Frontend establishes WebSocket connection on app startup and handles reconnection with exponential backoff.

**Tech Stack:** Gorilla WebSocket (Go), native WebSocket API (frontend), Redux for state management

---

## Prerequisites

- Gateway API server is running
- Frontend has Redux store configured
- Understanding of WebSocket protocol

---

### Task 1: Add WebSocket Support to Gateway

**Files:**
- Create: `internal/webui/websocket.go`
- Modify: `internal/webui/server.go`

**Step 1: Create WebSocket hub**

```go
package webui

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/Lichas/nanobot-go/internal/bus"
)

type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from Electron app
		return true
	},
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

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

func (h *WebSocketHub) Broadcast(messageType string, payload interface{}) {
	data, _ := json.Marshal(map[string]interface{}{
		"type":    messageType,
		"payload": payload,
	})
	h.broadcast <- data
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// Client messages not handled yet (one-way for now)
	}
}

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

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:  s.wsHub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
```

**Step 2: Add to server.go**

```go
import "github.com/gorilla/websocket"

type Server struct {
    // ... existing fields ...
    wsHub *WebSocketHub
}

func NewServer(cfg *config.Config, messageBus *bus.MessageBus) *Server {
    s := &Server{
        // ... existing ...
        wsHub: NewWebSocketHub(),
    }

    // Start WebSocket hub
    go s.wsHub.Run()

    // Subscribe to message bus
    if messageBus != nil {
        go s.subscribeToBus(messageBus)
    }

    return s
}

func (s *Server) registerRoutes() {
    // ... existing routes ...
    mux.HandleFunc("/ws", s.handleWebSocket)
}

func (s *Server) subscribeToBus(messageBus *bus.MessageBus) {
    // Subscribe to outbound messages
    for msg := range messageBus.Subscribe("outbound") {
        data, _ := json.Marshal(msg)
        s.wsHub.Broadcast("message", data)
    }
}
```

**Step 3: Add dependency**

```bash
go get github.com/gorilla/websocket
```

**Step 4: Commit**

```bash
git add go.mod go.sum internal/webui/websocket.go internal/webui/server.go
git commit -m "feat(api): add WebSocket support for real-time messaging"
```

---

### Task 2: Create Frontend WebSocket Client

**Files:**
- Create: `electron/src/renderer/services/websocket.ts`

**Step 1: Create WebSocket service**

```typescript
import { store } from '../store';
import { setStatus } from '../store';

type MessageHandler = (data: any) => void;

class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private handlers: Map<string, MessageHandler[]> = new Map();
  private url: string;

  constructor(url: string = 'ws://localhost:18890/ws') {
    this.url = url;
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    try {
      this.ws = new WebSocket(this.url);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.reconnectAttempts = 0;
        store.dispatch(setStatus({ state: 'running', port: 18890 }));
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.attemptReconnect();
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        store.dispatch(setStatus({ state: 'error', port: 18890, error: 'Connection failed' }));
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
      this.attemptReconnect();
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    setTimeout(() => {
      this.connect();
    }, delay);
  }

  private handleMessage(data: string): void {
    try {
      const message = JSON.parse(data);
      const handlers = this.handlers.get(message.type) || [];
      handlers.forEach((handler) => handler(message.payload));
    } catch (error) {
      console.error('Failed to parse WebSocket message:', error);
    }
  }

  on(type: string, handler: MessageHandler): () => void {
    const handlers = this.handlers.get(type) || [];
    handlers.push(handler);
    this.handlers.set(type, handlers);

    // Return unsubscribe function
    return () => {
      const updated = (this.handlers.get(type) || []).filter((h) => h !== handler);
      this.handlers.set(type, updated);
    };
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

export const wsClient = new WebSocketClient();
```

**Step 2: Commit**

```bash
git add electron/src/renderer/services/websocket.ts
git commit -m "feat(electron): add WebSocket client service"
```

---

### Task 3: Integrate WebSocket into App

**Files:**
- Modify: `electron/src/renderer/App.tsx`
- Modify: `electron/src/renderer/hooks/useGateway.ts`

**Step 1: Connect on app start**

In `App.tsx`:

```typescript
import { wsClient } from './services/websocket';

useEffect(() => {
  // Connect WebSocket
  wsClient.connect();

  // Subscribe to messages
  const unsubscribe = wsClient.on('message', (data) => {
    // Handle incoming messages
    console.log('Received message:', data);
    // Update Redux store if needed
  });

  return () => {
    unsubscribe();
    wsClient.disconnect();
  };
}, []);
```

**Step 2: Remove polling from useGateway**

```typescript
// Remove or reduce polling interval when WebSocket is connected
export function useGateway() {
  // ... existing code ...

  useEffect(() => {
    // Use WebSocket events instead of polling
    const unsubscribe = wsClient.on('message', () => {
      // Refresh sessions on new message
      fetchSessions();
    });

    return unsubscribe;
  }, []);
}
```

**Step 3: Commit**

```bash
git add electron/src/renderer/App.tsx electron/src/renderer/hooks/useGateway.ts
git commit -m "feat(electron): integrate WebSocket into app lifecycle"
```

---

### Task 4: Test and Optimize

**Step 1: Test scenarios**

1. Start app - verify WebSocket connects
2. Send message - verify received via WebSocket
3. Kill Gateway - verify reconnection attempts
4. Restart Gateway - verify auto-reconnect
5. Check network tab - verify no excessive polling

**Step 2: Performance metrics**

```bash
# Monitor WebSocket frames in DevTools
# Check latency reduction compared to polling
```

**Step 3: Update documentation**

Update status files and CHANGELOG.

**Step 4: Commit**

```bash
git add docs/ CHANGELOG.md
git commit -m "docs: update for WebSocket real-time support"
```

---

## Summary

After completing this plan:
- WebSocket connection replaces HTTP polling
- Messages delivered in real-time with lower latency
- Automatic reconnection on connection loss
- Reduced server load from fewer polling requests
