package handler

import (
	"net/http"
	"time"

	"jj/api"
	"jj/websocket"
)

func Routes() {
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
}
