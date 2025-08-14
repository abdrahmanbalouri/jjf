package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"jj/database" // Import the database package
	"jj/models"   // Import your models

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Client struct {
	Requests int
	LastSeen time.Time
}

var (
	clients = make(map[string]*Client)
	mu      sync.Mutex
)

// Utility Functions for API responses (can also be in a separate `utils` package)
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// authenticateUser checks for a valid session cookie and returns the user ID.
func authenticateUser(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		fmt.Println("99")
		return "", err
	}

	var userID string
	err = database.DB.QueryRow("SELECT id FROM users WHERE id = ?", cookie.Value).Scan(&userID)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return userID, nil
}

// RegisterHandler handles new user registration.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	type RegisterRequest struct {
		Nickname  string `json:"nickname"`
		Age       int    `json:"age"`
		Gender    string `json:"gender"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Nickname == "" || req.Email == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	id := uuid.New().String()
	_, err = database.DB.Exec(`
        INSERT INTO users (id, nickname, age, gender, first_name, last_name, email, password)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Nickname, req.Age, req.Gender, req.FirstName, req.LastName, req.Email, string(hashedPassword))
	if err != nil {
		if database.IsDuplicateKeyError(err) { // Use database.IsDuplicateKeyError
			respondWithError(w, http.StatusConflict, "Nickname or email already exists")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{"message": "User created successfully"})
}

// LoginHandler handles user login.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	type LoginRequest struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	var user struct {
		ID       string
		Nickname string
		Password string
	}
	err := database.DB.QueryRow(`
        SELECT id, nickname, password FROM users WHERE nickname = ? OR email = ?`,
		req.Identifier, req.Identifier).Scan(&user.ID, &user.Nickname, &user.Password)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	_, err = database.DB.Exec(`
        UPDATE users SET last_seen = CURRENT_TIMESTAMP, is_online = TRUE WHERE id = ?`,
		user.ID)
	if err != nil {
		log.Printf("Failed to update user status: %v", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    user.ID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400,
	})

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Login successful",
		"user":    models.User{ID: user.ID, Nickname: user.Nickname}, // Use models.User
	})
}

// LogoutHandler handles user logout.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not logged in")
		return
	}

	_, err = database.DB.Exec(`
        UPDATE users SET is_online = FALSE WHERE id = ?`,
		cookie.Value)
	if err != nil {
		log.Printf("Failed to update user status: %v", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUserHandler retrieves the currently authenticated user's information.
func GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticateUser(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var user models.User // Use models.User
	err = database.DB.QueryRow(`
        SELECT id, nickname FROM users WHERE id = ?`,
		userID).Scan(&user.ID, &user.Nickname)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

// GetUsersHandler retrieves a list of all users.
func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, nickname, is_online FROM users")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}
	defer rows.Close()

	var users []struct {
		models.User      // Embed models.User
		IsOnline    bool `json:"isOnline"`
	}
	for rows.Next() {
		var user struct {
			models.User
			IsOnline bool `json:"isOnline"`
		}
		if err := rows.Scan(&user.ID, &user.Nickname, &user.IsOnline); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to process users")
			return
		}
		users = append(users, user)
	}

	respondWithJSON(w, http.StatusOK, users)
}

// GetPostsHandler retrieves a list of all posts.
func GetPostsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
        SELECT p.id, p.title, p.content, p.category, p.created_at, u.nickname
        FROM posts p
        JOIN users u ON p.user_id = u.id
        ORDER BY p.created_at DESC`)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch posts")
		return
	}
	defer rows.Close()

	type Post struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		Category  string    `json:"category"`
		CreatedAt time.Time `json:"created_at"`
		Author    string    `json:"author"`
	}

	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt, &post.Author)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to process posts")
			return
		}
		posts = append(posts, post)
	}

	respondWithJSON(w, http.StatusOK, posts)
}

// GetPostHandler retrieves a single post by ID.
func GetPostHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 { // ["", "api", "posts", "123"]
		respondWithError(w, http.StatusNotFound, "Post not found")
		return
	}

	postID := parts[3]

	var post struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		Category  string    `json:"category"`
		CreatedAt time.Time `json:"created_at"`
		Author    string    `json:"author"`
	}
	err := database.DB.QueryRow(`
        SELECT p.id, p.title, p.content, p.category, p.created_at, u.nickname
        FROM posts p
        JOIN users u ON p.user_id = u.id
        WHERE p.id = ?`, postID).Scan(
		&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt, &post.Author)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch post")
		return
	}

	respondWithJSON(w, http.StatusOK, post)
}

// CreatePostHandler creates a new post.
func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticateUser(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	type PostRequest struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	var req PostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Title == "" || req.Content == "" || req.Category == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	postID := uuid.New().String()
	_, err = database.DB.Exec(`
        INSERT INTO posts (id, user_id, category, title, content)
        VALUES (?, ?, ?, ?, ?)`,
		postID, userID, req.Category, req.Title, req.Content)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create post")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"message": "Post created successfully",
		"post_id": postID,
	})
}

// GetCommentsHandler retrieves comments for a specific post.
func GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		respondWithError(w, http.StatusBadRequest, "Post ID required")
		return
	}
	fmt.Println(postID)

	rows, err := database.DB.Query(`
        SELECT c.id, c.content, c.created_at, u.nickname
        FROM comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.post_id = ?
        ORDER BY c.created_at ASC`, postID)
	if err != nil {
		fmt.Println("22")
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch comments")
		return
	}
	defer rows.Close()

	type Comment struct {
		ID        string    `json:"id"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
		Author    string    `json:"author"`
	}

	var comments []Comment
	for rows.Next() {
		var comment Comment
		err := rows.Scan(&comment.ID, &comment.Content, &comment.CreatedAt, &comment.Author)
		if err != nil {
			fmt.Println("22222")
			respondWithError(w, http.StatusInternalServerError, "Failed to process comments")
			return
		}
		comments = append(comments, comment)
	}

	respondWithJSON(w, http.StatusOK, comments)
}

// CreateCommentHandler creates a new comment for a post.
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticateUser(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	type CommentRequest struct {
		PostID  string `json:"post_id"`
		Content string `json:"content"`
	}

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.PostID == "" || req.Content == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	re := regexp.MustCompile(`<[^>]+>`)
	sanitizedContent := re.ReplaceAllString(req.Content, "")
	// Trim any extra whitespace
	sanitizedContent = strings.TrimSpace(sanitizedContent)

	commentID := uuid.New().String()
	_, err = database.DB.Exec(`
        INSERT INTO comments (id, post_id, user_id, content)
        VALUES (?, ?, ?, ?)`,
		commentID, req.PostID, userID, sanitizedContent)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create comment")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"message":    "Comment created successfully",
		"comment_id": commentID,
	})
}

// GetMessagesHandler retrieves private messages between two users.
func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticateUser(r)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	withUserId := r.URL.Query().Get("with")
	if withUserId == "" {
		respondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}

	limit := 10 // Default limit
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	query := `
        SELECT m.id, m.sender_id, m.content, m.created_at, u.nickname, m.is_read
        FROM private_messages m
        JOIN users u ON m.sender_id = u.id
        WHERE (m.sender_id = ? AND m.receiver_id = ?) OR (m.sender_id = ? AND m.receiver_id = ?)
    `
	args := []interface{}{userID, withUserId, withUserId, userID}

	if before := r.URL.Query().Get("before"); before != "" {
		query += ` AND m.created_at < ?`
		args = append(args, before)
	}

	query += ` ORDER BY m.created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch messages")
		return
	}
	defer rows.Close()

	type Message struct {
		ID        string    `json:"id"`
		SenderId  string    `json:"senderId"`
		Content   string    `json:"content"`
		Timestamp time.Time `json:"timestamp"`
		Sender    string    `json:"sender"`
		IsRead    bool      `json:"isRead"`
	}

	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.SenderId, &msg.Content, &msg.Timestamp, &msg.Sender, &msg.IsRead); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to process messages")
			return
		}
		messages = append(messages, msg)
	}

	// Reverse messages to maintain ascending order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	respondWithJSON(w, http.StatusOK, messages)
}

func RateLimitMiddleware(next http.HandlerFunc, limit int, window time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		mu.Lock()
		defer mu.Unlock()

		c, exists := clients[ip]
		if !exists || time.Since(c.LastSeen) > window {
			clients[ip] = &Client{Requests: 1, LastSeen: time.Now()}
		} else {
			c.Requests++
			c.LastSeen = time.Now()
			if c.Requests > limit {
			respondWithError(w, http.StatusMethodNotAllowed, "Allot of requset")
				return
			}
		}

		next(w, r)
	}
}
