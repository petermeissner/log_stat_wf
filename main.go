package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Configure logger with custom timestamp format
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&logWriter{})

	// Define command-line flags
	host := flag.String("host", "localhost", "Host to listen on")
	port := flag.String("port", "3001", "Port to listen on")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	listenAddr := *host + ":" + *port

	log.Println("=== WildFly Log Receiver ===")
	log.Println("=== Starting Server on " + listenAddr + " ===")

	// Create log stat store
	store := NewLogStatStore()

	// Listen on the specified address
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}
	defer listener.Close()

	log.Println("=== Server listening, waiting for WildFly logs... === ")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n\n=== Shutting down ===")
		store.PrintSummary()
		listener.Close()
		os.Exit(0)
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Accept error:", err)
		}

		log.Printf("=== New connection from %s ===", conn.RemoteAddr())

		// Handle each connection in a goroutine
		go handleConnection(conn, *verbose, store)
	}
}

func handleConnection(conn net.Conn, verbose bool, store *LogStatStore) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()

		handleLogEntry(line, store)
	}

	if err := scanner.Err(); err != nil {
		if verbose {
			log.Printf("Connection error from %s: %v\n", remoteAddr, err)
		}
	}

	if verbose {
		log.Printf("Connection closed: %s\n", remoteAddr)
	}
}
