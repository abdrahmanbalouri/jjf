package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"jj/api" // Your API handlers
	"jj/database"
	"jj/websocket"
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

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API Routes (using functions from the `api` package)
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

	http.HandleFunc("/ws", websocket.WsHandler)

	// SPA (Single Page Application) fallback
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		       fmt.Println(r.URL.Path)
		 if (r.URL.Path!= "/"){
               
				// fmt.Println("54445")
		  http.Redirect(w, r, "/", http.StatusSeeOther) // 303
			return
		 }
		http.ServeFile(w, r, "./static/index.html")
	})

	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
