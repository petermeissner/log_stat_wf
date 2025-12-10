package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Define command-line flags
	host := flag.String("host", "localhost", "Host to listen on")
	tcpPort := flag.String("tcp-port", "3001", "TCP port for log receiver")
	httpPort := flag.String("http-port", "3000", "HTTP port for web interface")
	dbPath := flag.String("db-path", "log_stat.db", "Path to SQLite database file")
	bucketSize := flag.Duration("bucket-size", 1*time.Minute, "Time bucket size (1m, 5m, 10m, 15m, 20m, 30m, 60m)")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	// Setup logging with rotation (console + rotating file)
	setupLogging("log_stat.log")

	tcpAddr := *host + ":" + *tcpPort
	httpAddr := *host + ":" + *httpPort

	// Validate bucket size
	validSizes := map[time.Duration]bool{
		1 * time.Minute:  true,
		5 * time.Minute:  true,
		10 * time.Minute: true,
		15 * time.Minute: true,
		20 * time.Minute: true,
		30 * time.Minute: true,
		60 * time.Minute: true,
	}
	if !validSizes[*bucketSize] {
		log.Fatal("Invalid bucket size. Allowed values: 1m, 5m, 10m, 15m, 20m, 30m, 60m")
	}

	log.Println("=== WildFly Log Receiver/Reporter ===")
	log.Println("=== Starting LogIngest Server on " + tcpAddr + " ===")
	log.Println("=== Starting LogStat HTTP Server on " + httpAddr + " ===")
	log.Printf("=== Bucket size: %v ===\n", *bucketSize)

	// Create log stat store with bucket size
	store := NewLogStatStore(*bucketSize, *dbPath, *verbose)

	// Initialize database
	if err := store.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Start TCP listener for logs
	listener, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		log.Fatal("Failed to listen on TCP:", err)
	}
	defer listener.Close()

	// Start HTTP server
	go startHTTPServer(httpAddr, store)

	// Start periodic flush to database
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			store.FlushToDb()
		}
	}()

	log.Println("=== Servers listening ===")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n\n=== Shutting down ===")
		store.PrintSummary()
		store.FlushToDb()
		listener.Close()
		os.Exit(0)
	}()

	// Accept TCP connections
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

		store.handleJsonLogEntry(line)
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
