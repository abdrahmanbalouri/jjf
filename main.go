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

	"jj/api"
	"jj/database"
	"jj/websocket"
)

func main() {
	// Initialize Database
	err := database.InitDB("./forum.db")
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

	// Static file server
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API Routes
	http.HandleFunc("/api/register", api.RateLimitMiddleware(api.RegisterHandler, 5, time.Minute))
	http.HandleFunc("/api/login", api.RateLimitMiddleware(api.LoginHandler, 5, time.Minute))
	http.HandleFunc("/api/logout", api.LogoutHandler)
	http.HandleFunc("/api/users/me", api.GetCurrentUserHandler)
	http.HandleFunc("/api/users", api.GetUsersHandler)
	http.HandleFunc("/api/posts", api.GetPostsHandler)
	http.HandleFunc("/api/posts/create", api.RateLimitMiddleware(api.CreatePostHandler, 5, time.Minute))
	http.HandleFunc("/api/posts/{id}", api.GetPostHandler)
	http.HandleFunc("/api/getcomments", api.GetCommentsHandler)
	http.HandleFunc("/api/comments", api.RateLimitMiddleware(api.CreateCommentHandler, 5, time.Minute))
	http.HandleFunc("/api/messages", api.GetMessagesHandler)
	http.HandleFunc("/api/posts/forcreate", api.GetPostsHandlerfor)

	http.HandleFunc("/api/auto", api.Auto)
	http.HandleFunc("/ws", websocket.WsHandler)

	// SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./static/index.html")
	})

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
