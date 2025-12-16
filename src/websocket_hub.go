package main

import (
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from log ingestion
	broadcast chan *RawLogEntry

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Maximum number of clients
	maxClients int

	// Mutex for client map
	mutex sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub(maxClients int) *Hub {
	return &Hub{
		broadcast:  make(chan *RawLogEntry, 1000), // Buffer for incoming log messages
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		maxClients: maxClients,
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Check if we've reached the max client limit
	if len(h.clients) >= h.maxClients {
		log.Printf("Maximum client limit reached (%d), rejecting new client", h.maxClients)
		close(client.send)
		client.conn.Close()
		return
	}

	h.clients[client] = true
	log.Printf("Client registered, total clients: %d/%d", len(h.clients), h.maxClients)
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
		log.Printf("Client unregistered, remaining clients: %d/%d", len(h.clients), h.maxClients)
	}
}

// broadcastMessage sends a message to all connected clients
func (h *Hub) broadcastMessage(message *RawLogEntry) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Send to all clients (each client will filter based on their subscription)
	for client := range h.clients {
		// Process message in goroutine to avoid blocking other clients
		go client.ProcessMessage(message)
	}
}

// BroadcastLog sends a log entry to the hub for broadcasting
// This is called from the log ingestion pipeline
func (h *Hub) BroadcastLog(entry *RawLogEntry) {
	select {
	case h.broadcast <- entry:
		// Message queued successfully
	default:
		// Broadcast channel is full, drop message
		log.Printf("Hub broadcast channel full, dropping message")
	}
}

// clientCount returns the current number of connected clients
func (h *Hub) clientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return map[string]interface{}{
		"connected_clients": len(h.clients),
		"max_clients":       h.maxClients,
		"broadcast_buffer":  len(h.broadcast),
	}
}
