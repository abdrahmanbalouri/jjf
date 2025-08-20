package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jj/database"
	"jj/handler"
)

func main() {
	// Initialize Database
	err := database.InitDB("./db/forum.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB() // Ensure database connection is closed

	// Create tables
	err = database.CreateTables()
	if err != nil {
		log.Fatalf("Failed to create database tables: %v", err)
	}

	// Set up HTTP server
	server := &http.Server{
		Addr: "0.0.0.0:8080",
	}
	handler.Routes()

	// Start server in a goroutine
	go func() {
		fmt.Println("Server started at http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Handle OS signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop // Wait for signal
	log.Println("Received shutdown signal, initiating graceful shutdown...")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	} else {
		log.Println("Server shut down gracefully")
	}

	// Database is closed via defer database.CloseDB()
	log.Println("Program exiting")
}
