package main

import (
	"fmt"
	"log"
	"net/http"

	"jj/api"       // Your API handlers
	"jj/database"  // Your database connection and schema
	"jj/websocket" // Your WebSocket handlers
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
	fs := http.FileServer(http.Dir("./static"))

	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API routes
	http.HandleFunc("/api/register", api.RegisterHandler)
	http.HandleFunc("/api/login", api.LoginHandler)
	http.HandleFunc("/api/logout", api.LogoutHandler)
	http.HandleFunc("/api/users/me", api.GetCurrentUserHandler)
	http.HandleFunc("/api/users", api.GetUsersHandler)
	http.HandleFunc("/api/posts", api.GetPostsHandler)
	http.HandleFunc("/api/posts/create", api.CreatePostHandler)
	http.HandleFunc("/api/posts/", api.GetPostHandler)       // تحتاج التعامل مع /api/posts/{id} داخل الفنكشن
	http.HandleFunc("/api/comments", api.GetCommentsHandler) // غادي تفرق بين GET و POST داخل الفنكشن
	http.HandleFunc("/api/messages", api.GetMessagesHandler)

	// WebSocket
	http.HandleFunc("/ws", websocket.WsHandler)

	// SPA fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	// Start server
	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
