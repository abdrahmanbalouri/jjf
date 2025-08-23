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

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"jj/database"
	"jj/models"
)

type Client struct {
	Requests int
	LastSeen time.Time
}

var (
	clients = make(map[string]*Client)
	mu      sync.Mutex
)
var Lougout bool

// Utility Functions for API responses (can also be in a separate `utils` package)
func RespondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// RegisterHandler handles new user registration.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")

	if k != "*/*" {

		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "POST" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
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
		RespondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	if !IsGmail(req.Email) {
		RespondWithError(w, http.StatusBadRequest, "handle your gmail plzz")
		return

	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Password = strings.TrimSpace(req.Password)

	if (len(req.Nickname) < 2 || len(req.Nickname) > 10) || (len(req.FirstName) < 2 || len(req.FirstName) > 10) || (len(req.LastName) < 2 || len(req.LastName) > 10) || (len(req.Password) < 2 || len(req.Password) > 10) || (req.Age > 100 || req.Age < 20) {
		RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	id := uuid.New().String()
	nickname := models.Skip(req.Nickname)
	Firstname := models.Skip(req.FirstName)
	Lastname := models.Skip(req.LastName)

	_, err = database.DB.Exec(`
        INSERT INTO users (id, nickname, age, gender, first_name, last_name, email, password)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, nickname, req.Age, req.Gender, Firstname, Lastname, req.Email, string(hashedPassword))
	if err != nil {
		if database.IsDuplicateKeyError(err) { // Use database.IsDuplicateKeyError
			RespondWithError(w, http.StatusConflict, "Nickname or email already exists")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	respondWithJSON(w, http.StatusCreated, map[string]string{"message": "User created successfully"})
}

// LoginHandler handles user login.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	 fmt.Println(r.Method)
	 k := r.Header.Get("Accept")
	 if k != "*/*" {
		 
		 http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		 return
		}
		if r.Method != "POST" {
			RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
			return
		}
		type LoginRequest struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	var user struct {
		ID       string
		Nickname string
		Password string
	}
	token := uuid.New().String()

	err := database.DB.QueryRow(`
        SELECT id, nickname, password FROM users WHERE nickname = ? OR email = ?`,
		req.Identifier, req.Identifier).Scan(&user.ID, &user.Nickname, &user.Password)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Invalid credentials")
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
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	_, err1 := database.DB.Exec(`
    UPDATE users
    SET token = ?
    WHERE id = ?`,
		token, user.ID,
	)
	if err1 != nil {
		log.Println("Failed to update token:", err)
		return
	}
	Lougout = false
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Login successful",
		"user":    models.User{ID: user.ID, Nickname: user.Nickname}, // Use models.User
	})
}

// LogoutHandler handles user logout.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")

	if k != "*/*" {

		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}

	if r.Method != "POST" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	withUserId := r.URL.Query().Get("with")
	Lougout = true

	_, err := database.DB.Exec(`
        UPDATE users SET is_online = FALSE WHERE id = ?`,
		withUserId)
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
	_, err2 := database.DB.Exec(`
    UPDATE users
    SET token = NULL
    WHERE id = ?`,
		withUserId, // l'user li kay logout
	)
	if err2 != nil {
		log.Println("Failed to clear token in DB:", err)
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUserHandler retrieves the currently authenticated user's information.
func GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	if k != "*/*" {
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	userID, err := authenticateUser(r)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var user models.User // Use models.User
	err = database.DB.QueryRow(`
        SELECT id, nickname FROM users WHERE id = ?`,
		userID).Scan(&user.ID, &user.Nickname)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get user info")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

// GetUsersHandler retrieves a list of all users.
func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")

	if k != "*/*" {

		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}

	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	rows, err := database.DB.Query("SELECT id, nickname, is_online FROM users")
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to fetch users")
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
			RespondWithError(w, http.StatusInternalServerError, "Failed to process users")
			return
		}
		users = append(users, user)
	}

	respondWithJSON(w, http.StatusOK, users)
}

func GetPostsHandlerfor(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	if k != "*/*" {
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	row := database.DB.QueryRow(`
        SELECT p.id, p.title, p.content, p.category, p.created_at, u.nickname
        FROM posts p
        JOIN users u ON p.user_id = u.id
        ORDER BY p.created_at DESC
        LIMIT 1`)

	type Post struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		Category  string    `json:"category"`
		CreatedAt time.Time `json:"created_at"`
		Author    string    `json:"author"`
	}

	var post Post
	err := row.Scan(&post.ID, &post.Title, &post.Content, &post.Category, &post.CreatedAt, &post.Author)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithJSON(w, http.StatusOK, nil) // ma kaynsh post
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to fetch last post")
		return
	}

	respondWithJSON(w, http.StatusOK, post)
}

// GetPostsHandler retrieves a list of all posts.
func GetPostsHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	
	if k != "*/*" {
		
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	limit := 10
	offset := r.URL.Query().Get("with")

	rows, err := database.DB.Query(`
    SELECT p.id, p.title, p.content, p.category, p.created_at, u.nickname
    FROM posts p
    JOIN users u ON p.user_id = u.id
    ORDER BY p.created_at DESC
    LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		log.Println("Query error:", err)
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
			RespondWithError(w, http.StatusInternalServerError, "Failed to process posts")
			return
		}
		posts = append(posts, post)
	}

	respondWithJSON(w, http.StatusOK, posts)
}

// GetPostHandler retrieves a single post by ID.
func GetPostHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	
	if k != "*/*" {
		
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 { // ["", "api", "posts", "123"]
		RespondWithError(w, http.StatusNotFound, "Post not found")
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
			RespondWithError(w, http.StatusNotFound, "Post not found")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to fetch post")
		return
	}

	respondWithJSON(w, http.StatusOK, post)
}

// CreatePostHandler creates a new post.
func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	
	if k != "*/*" {
		
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "POST" {
		fmt.Println("222222222")
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	userID, err := authenticateUser(r)
	if err != nil {
		fmt.Println("2222")
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	type PostRequest struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}

	var req PostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if (len(req.Title) < 5 || len(req.Title) > 50) || (len(req.Content) < 5 || len(req.Content) > 50) || req.Category == "" {
		RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	title := models.Skip(req.Title)
	content := models.Skip(req.Content)

	postID := uuid.New().String()
	_, err = database.DB.Exec(`
        INSERT INTO posts (id, user_id, category, title, content)
        VALUES (?, ?, ?, ?, ?)`,
		postID, userID, req.Category, title, content)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create post")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"message": "Post created successfully",
		"post_id": postID,
	})
}

// GetCommentsHandler retrieves comments for a specific post.
func GetCommentsHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	
	if k != "*/*" {
		
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		RespondWithError(w, http.StatusBadRequest, "Post ID required")
		return
	}

	rows, err := database.DB.Query(`
        SELECT c.id, c.content, c.created_at, u.nickname
        FROM comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.post_id = ?
        ORDER BY c.created_at ASC`, postID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to fetch comments")
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
			RespondWithError(w, http.StatusInternalServerError, "Failed to process comments")
			return
		}
		comments = append(comments, comment)
	}

	respondWithJSON(w, http.StatusOK, comments)
}

// CreateCommentHandler creates a new comment for a post.
func CreateCommentHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	
	if k != "*/*" {
		
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "POST" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	userID, err := authenticateUser(r)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	type CommentRequest struct {
		PostID  string `json:"post_id"`
		Content string `json:"content"`
	}

	var req CommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.PostID == "" || strings.TrimSpace(req.Content) == "" {
		RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	if len(req.Content) < 3 || len(req.Content) > 30 {
		RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}
	sanitizedContent := models.Skip(req.Content)

	commentID := uuid.New().String()
	_, err = database.DB.Exec(`
        INSERT INTO comments (id, post_id, user_id, content)
        VALUES (?, ?, ?, ?)`,
		commentID, req.PostID, userID, sanitizedContent)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create comment")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{
		"message":    "Comment created successfully",
		"comment_id": commentID,
	})
}

// GetMessagesHandler retrieves private messages between two users.
func GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	if k != "*/*" {
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	userID, err := authenticateUser(r)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	withUserId := r.URL.Query().Get("with")
	if withUserId == "" {
		RespondWithError(w, http.StatusBadRequest, "Missing user ID")
		return
	}

	limit := 10 // Default limit
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	// Récupère le paramètre "offset" de l'URL
	offset := 0
	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// Voici le changement clé : la requête SQL utilise "LIMIT" et "OFFSET"
	query := `
        SELECT m.id, m.sender_id, m.content, m.created_at, u.nickname, m.is_read
        FROM private_messages m
        JOIN users u ON m.sender_id = u.id
        WHERE (m.sender_id = ? AND m.receiver_id = ?) OR (m.sender_id = ? AND m.receiver_id = ?)
        ORDER BY m.idss DESC, m.id DESC
        LIMIT ? OFFSET ?
    `
	args := []interface{}{userID, withUserId, withUserId, userID, limit, offset}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to fetch messages")
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
			RespondWithError(w, http.StatusInternalServerError, "Failed to process messages")
			return
		}
		messages = append(messages, msg)
	}

	// Inverse les messages pour les renvoyer dans l'ordre chronologique (du plus ancien au plus récent)
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

		if !exists {
			clients[ip] = &Client{Requests: 1, LastSeen: time.Now()}
		} else {
			if time.Since(c.LastSeen) > window {
				c.Requests = 1
				c.LastSeen = time.Now()
			} else {
				c.Requests++
				if c.Requests > limit {
					c.Requests = 0
					c.LastSeen = time.Now()
					RespondWithError(w, http.StatusMethodNotAllowed, "A lot of requests")
					return
				}
			}
		}

		next(w, r)
	}
}

func authenticateUser(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return "", err
	}

	var userID string
	err = database.DB.QueryRow("SELECT id FROM users WHERE token = ?", cookie.Value).Scan(&userID)
	if err != nil {
		// fmt.Println(err)
		return "", err
	}

	return userID, nil
}

func Auto(w http.ResponseWriter, r *http.Request) {
	k := r.Header.Get("Accept")
	if k != "*/*" {
		http.Redirect(w, r, "/", http.StatusSeeOther) // 303
		return
	}
	if r.Method != "GET" {
		RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	withUserId := r.URL.Query().Get("with")
	var userid string
	err := database.DB.QueryRow(`SELECT id FROM users WHERE token = ?`, withUserId).Scan(&userid)
	if err != nil {
		respondWithJSON(w, http.StatusUnauthorized, map[string]string{"message": "Invalid session"})
		return
	}
	respondWithJSON(w, http.StatusOK, userid)
}

func IsGmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.]{5,29}@gmail\.com$`)
	return re.MatchString(email)
}
