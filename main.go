package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"jj/api"        // Your API handlers
	"jj/database"   // Your database connection and schema
	"jj/websocket"  // Your WebSocket handlers

	"github.com/gorilla/mux"
)

func main() {
    // 1. Initialize Database
    err := database.InitDB("./forum.db")
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer database.CloseDB() // Ensure database connection is closed

    // 2. Create tables
    err = database.CreateTables()
    if err != nil {
        log.Fatalf("Failed to create database tables: %v", err)
    }

    // 3. Setup Router
    r := mux.NewRouter()

    // Serve static files
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

    // API Routes (using functions from the `api` package)
    r.HandleFunc("/api/register", api.RegisterHandler).Methods("POST")
    r.HandleFunc("/api/login", api.LoginHandler).Methods("POST")
    r.HandleFunc("/api/logout", api.LogoutHandler).Methods("POST")
    r.HandleFunc("/api/users/me", api.GetCurrentUserHandler).Methods("GET")
    r.HandleFunc("/api/users", api.GetUsersHandler).Methods("GET")
    r.HandleFunc("/api/posts", api.GetPostsHandler).Methods("GET")
    r.HandleFunc("/api/posts/create", api.CreatePostHandler).Methods("POST")
    r.HandleFunc("/api/posts/{id}", api.GetPostHandler).Methods("GET")
    r.HandleFunc("/api/comments", api.GetCommentsHandler).Methods("GET")
    r.HandleFunc("/api/comments", api.CreateCommentHandler).Methods("POST")
    r.HandleFunc("/api/messages", api.GetMessagesHandler).Methods("GET")

    // WebSocket Route (using the handler from the `websocket` package)
    r.HandleFunc("/ws", websocket.WsHandler)

    // SPA (Single Page Application) fallback
    r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "./static/index.html")
    })

    // 4. Start the HTTP Server
    srv := &http.Server{
        Handler:      r,
        Addr:         "0.0.0.0:8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    fmt.Println("Server started at http://localhost:8080")
    log.Fatal(srv.ListenAndServe())
}