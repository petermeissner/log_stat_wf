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
	// Configure logger with custom timestamp format
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&logWriter{})

	// Define command-line flags
	host := flag.String("host", "localhost", "Host to listen on")
	tcpPort := flag.String("tcp-port", "3001", "TCP port for log receiver")
	httpPort := flag.String("http-port", "3000", "HTTP port for web interface")
	dbPath := flag.String("db-path", "log_stat.db", "Path to SQLite database file")
	flushInterval := flag.Duration("flush-interval", 5*time.Minute, "Interval for flushing data to database")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	tcpAddr := *host + ":" + *tcpPort
	httpAddr := *host + ":" + *httpPort

	log.Println("=== WildFly Log Receiver/Reporter ===")
	log.Println("=== Starting LogIngest Server on " + tcpAddr + " ===")
	log.Println("=== Starting LogStat HTTP Server on " + httpAddr + " ===")

	// Create log stat store
	store := NewLogStatStore()

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
		ticker := time.NewTicker(*flushInterval)
		defer ticker.Stop()

		for range ticker.C {
			store.FlushToDb(*dbPath)
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
		store.FlushToDb(*dbPath)
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
