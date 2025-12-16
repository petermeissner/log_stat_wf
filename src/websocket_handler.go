package main

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// SetupWebSocketRoutes adds WebSocket routes to the Fiber app
func SetupWebSocketRoutes(app *fiber.App, hub *Hub) {
	// WebSocket upgrade middleware
	app.Use("/ws", func(c *fiber.Ctx) error {
		// Check if it's a WebSocket upgrade request
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket endpoint
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		handleWebSocketConnection(c, hub)
	}))
}

// handleWebSocketConnection handles a new WebSocket connection
func handleWebSocketConnection(conn *websocket.Conn, hub *Hub) {
	// Create new client
	client := NewClient(hub, conn)

	// Register client with hub
	hub.register <- client

	// Start client read and write pumps
	go client.writePump()
	client.readPump() // This blocks until connection closes
}
