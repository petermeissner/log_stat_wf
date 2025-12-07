package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

//go:embed web/*
var webFiles embed.FS

func startHTTPServer(addr string, store *LogStatStore) {
	app := fiber.New(fiber.Config{
		AppName: "WildFly Log Statistics",
	})

	// API endpoint for stats
	app.Get("/api/stats", func(c *fiber.Ctx) error {
		stats := store.GetAll()
		return c.JSON(stats)
	})

	// Serve embedded static files (CSS, JS)
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(webFiles),
		PathPrefix: "web",
		Browse:     false,
	}))

	log.Printf("Fiber HTTP server starting on %s\n", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("HTTP server error: %v\n", err)
	}
}
