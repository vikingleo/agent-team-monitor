package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/liaoweijun/agent-team-monitor/pkg/monitor"
)

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	collector   *monitor.Collector
	clients     map[*WebSocketClient]bool
	broadcast   chan []byte
	register    chan *WebSocketClient
	unregister  chan *WebSocketClient
}

// WebSocketClient represents a WebSocket client
type WebSocketClient struct {
	hub  *WebSocketHub
	conn http.ResponseWriter
	send chan []byte
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(collector *monitor.Collector) *WebSocketHub {
	return &WebSocketHub{
		collector:  collector,
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("WebSocket client connected. Total: %d", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("WebSocket client disconnected. Total: %d", len(h.clients))
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}

		case <-ticker.C:
			// Broadcast state updates to all connected clients
			state := h.collector.GetState()
			data, err := json.Marshal(state)
			if err != nil {
				log.Printf("Error marshaling state: %v", err)
				continue
			}
			h.broadcast <- data
		}
	}
}
