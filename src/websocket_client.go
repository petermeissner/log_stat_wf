package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"golang.org/x/time/rate"
)

// Client represents a WebSocket client connection
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte // Buffered channel of outbound messages (500 capacity)
	subscription *ClientSubscription
	filter       *MessageFilter

	// Rate limiting
	rateLimiter *rate.Limiter

	// Batching
	batchBuffer [][]byte
	batchTimer  *time.Timer
	batchMutex  sync.Mutex

	// Statistics
	messagesQueued  int
	messagesDropped int
	statsMutex      sync.RWMutex
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	// Start with default subscription (INFO and above)
	defaultSub := GetDefaultSubscription()
	filter, err := NewMessageFilter(defaultSub)
	if err != nil {
		log.Printf("Error creating default filter: %v", err)
		filter = nil
	}

	client := &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, 500), // 500 message buffer
		subscription: defaultSub,
		filter:       filter,
		batchBuffer:  make([][]byte, 0, 10), // Initial batch capacity
	}

	// Set up rate limiter (0 = unlimited by default)
	if defaultSub.MaxMessagesPerSecond > 0 {
		client.rateLimiter = rate.NewLimiter(rate.Limit(defaultSub.MaxMessagesPerSecond), defaultSub.MaxMessagesPerSecond)
	}

	// Start batch timer if batching is enabled
	if defaultSub.BatchTimeoutMs > 0 {
		client.batchTimer = time.NewTimer(time.Duration(defaultSub.BatchTimeoutMs) * time.Millisecond)
		go client.handleBatchTimeout()
	}

	return client
}

// UpdateSubscription updates the client's subscription and recompiles filters
func (c *Client) UpdateSubscription(sub *ClientSubscription) error {
	filter, err := NewMessageFilter(sub)
	if err != nil {
		return err
	}

	c.subscription = sub
	c.filter = filter

	// Update rate limiter
	if sub.MaxMessagesPerSecond > 0 {
		c.rateLimiter = rate.NewLimiter(rate.Limit(sub.MaxMessagesPerSecond), sub.MaxMessagesPerSecond)
	} else {
		c.rateLimiter = nil
	}

	// Update batching
	if sub.BatchTimeoutMs > 0 {
		if c.batchTimer == nil {
			c.batchTimer = time.NewTimer(time.Duration(sub.BatchTimeoutMs) * time.Millisecond)
			go c.handleBatchTimeout()
		}
	} else {
		if c.batchTimer != nil {
			c.batchTimer.Stop()
			c.batchTimer = nil
		}
	}

	return nil
}

// ProcessMessage filters and transforms a message for this client
func (c *Client) ProcessMessage(raw *RawLogEntry) {
	// Check if message matches filters
	if c.filter != nil && !c.filter.Matches(raw) {
		return
	}

	// Check rate limit
	if c.rateLimiter != nil {
		if !c.rateLimiter.Allow() {
			c.statsMutex.Lock()
			c.messagesDropped++
			c.statsMutex.Unlock()
			return
		}
	}

	// Transform message
	msg := TransformMessage(raw, c.filter)

	// Serialize to JSON
	data, err := json.Marshal(ServerMessage{
		Type: "log",
		Data: msg,
	})
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	// Handle batching
	if c.subscription.BatchTimeoutMs > 0 {
		c.addToBatch(data)
	} else {
		c.sendMessage(data)
	}
}

// sendMessage sends a message to the client's send channel
func (c *Client) sendMessage(data []byte) {
	select {
	case c.send <- data:
		c.statsMutex.Lock()
		c.messagesQueued++
		c.statsMutex.Unlock()
	default:
		// Channel full, drop message
		c.statsMutex.Lock()
		c.messagesDropped++
		c.statsMutex.Unlock()
		log.Printf("Client send buffer full, dropping message")
	}
}

// addToBatch adds a message to the batch buffer
func (c *Client) addToBatch(data []byte) {
	c.batchMutex.Lock()
	defer c.batchMutex.Unlock()

	c.batchBuffer = append(c.batchBuffer, data)
}

// flushBatch sends the accumulated batch
func (c *Client) flushBatch() {
	c.batchMutex.Lock()
	defer c.batchMutex.Unlock()

	if len(c.batchBuffer) == 0 {
		return
	}

	// Parse all buffered messages
	messages := make([]*LogMessage, 0, len(c.batchBuffer))
	for _, data := range c.batchBuffer {
		var serverMsg ServerMessage
		if err := json.Unmarshal(data, &serverMsg); err != nil {
			continue
		}
		if logMsg, ok := serverMsg.Data.(*LogMessage); ok {
			messages = append(messages, logMsg)
		}
	}

	// Create batch message
	batchMsg := ServerMessage{
		Type: "batch",
		Data: BatchMessage{
			Messages: messages,
			Count:    len(messages),
		},
	}

	// Serialize and send
	data, err := json.Marshal(batchMsg)
	if err != nil {
		log.Printf("Error marshaling batch: %v", err)
		return
	}

	c.sendMessage(data)

	// Clear buffer
	c.batchBuffer = c.batchBuffer[:0]
}

// handleBatchTimeout handles periodic batch flushing
func (c *Client) handleBatchTimeout() {
	for {
		if c.batchTimer == nil {
			return
		}

		<-c.batchTimer.C

		c.flushBatch()

		// Reset timer
		if c.subscription.BatchTimeoutMs > 0 {
			c.batchTimer.Reset(time.Duration(c.subscription.BatchTimeoutMs) * time.Millisecond)
		} else {
			return
		}
	}
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var clientMsg ClientMessage
		err := c.conn.ReadJSON(&clientMsg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleClientMessage(&clientMsg)
	}
}

// writePump writes messages from the send channel to the WebSocket connection
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		message, ok := <-c.send
		if !ok {
			// Hub closed the channel
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}

		c.statsMutex.Lock()
		c.messagesQueued--
		c.statsMutex.Unlock()
	}
}

// handleClientMessage processes messages from the client
func (c *Client) handleClientMessage(msg *ClientMessage) {
	switch msg.Action {
	case "subscribe":
		var sub ClientSubscription
		if err := json.Unmarshal(msg.Data, &sub); err != nil {
			c.sendError("invalid_subscription", "Invalid subscription format")
			return
		}

		if err := c.UpdateSubscription(&sub); err != nil {
			c.sendError("filter_error", err.Error())
			return
		}

		c.sendAck("subscribed")

	case "update":
		var sub ClientSubscription
		if err := json.Unmarshal(msg.Data, &sub); err != nil {
			c.sendError("invalid_subscription", "Invalid subscription format")
			return
		}

		if err := c.UpdateSubscription(&sub); err != nil {
			c.sendError("filter_error", err.Error())
			return
		}

		c.sendAck("updated")

	case "ping":
		c.sendPong()

	case "stats":
		c.sendStats()

	default:
		c.sendError("unknown_action", "Unknown action: "+msg.Action)
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	data, err := json.Marshal(ServerMessage{
		Type: "error",
		Data: ErrorMessage{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return
	}
	c.sendMessage(data)
}

// sendAck sends an acknowledgment message
func (c *Client) sendAck(message string) {
	data, err := json.Marshal(ServerMessage{
		Type: "ack",
		Data: map[string]string{"message": message},
	})
	if err != nil {
		return
	}
	c.sendMessage(data)
}

// sendPong sends a pong response
func (c *Client) sendPong() {
	data, err := json.Marshal(ServerMessage{
		Type: "pong",
		Data: map[string]int64{"timestamp": time.Now().Unix()},
	})
	if err != nil {
		return
	}
	c.sendMessage(data)
}

// sendStats sends client statistics
func (c *Client) sendStats() {
	c.statsMutex.RLock()
	stats := StatsMessage{
		Connected:      c.hub.clientCount(),
		TotalClients:   c.hub.maxClients,
		MessagesQueued: len(c.send),
		Dropped:        c.messagesDropped,
	}
	c.statsMutex.RUnlock()

	data, err := json.Marshal(ServerMessage{
		Type: "stats",
		Data: stats,
	})
	if err != nil {
		return
	}
	c.sendMessage(data)
}
