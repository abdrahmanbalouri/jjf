package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
    _ "github.com/mattn/go-sqlite3"
    "golang.org/x/crypto/bcrypt"
)

// Data Structures
type User struct {
    ID       string `json:"id"`
    Nickname string `json:"nickname"`
}

type Client struct {
    conn   *websocket.Conn
    userId string
}

// Global Variables
var (
    db           *sql.DB
    clients      = make(map[*Client]bool)
    clientsMutex sync.Mutex
    upgrader     = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
)

func main() {
    var err error
    db, err = sql.Open("sqlite3", "./forum.db")
    if err != nil {
        log.Fatal("Failed to open database: ", err)
    }
    defer db.Close()

    err = createTables()
    if err != nil {
        log.Fatal("Failed to create tables: ", err)
    }

    r := mux.NewRouter()

    // Static files
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

    // API Routes
    r.HandleFunc("/api/register", registerHandler).Methods("POST")
    r.HandleFunc("/api/login", loginHandler).Methods("POST")
    r.HandleFunc("/api/logout", logoutHandler).Methods("POST")
    r.HandleFunc("/api/users/me", getCurrentUserHandler).Methods("GET")
    r.HandleFunc("/api/users", getUsersHandler).Methods("GET")
    r.HandleFunc("/api/posts", getPostsHandler).Methods("GET")
    r.HandleFunc("/api/posts/create", createPostHandler).Methods("POST")
    r.HandleFunc("/api/posts/{id}", getPostHandler).Methods("GET")
    r.HandleFunc("/api/comments", getCommentsHandler).Methods("GET")
    r.HandleFunc("/api/comments", createCommentHandler).Methods("POST")
    r.HandleFunc("/api/messages", getMessagesHandler).Methods("GET")

    // WebSocket
    r.HandleFunc("/ws", wsHandler)

    // SPA
    r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "./static/index.html")
    })

    // Server configuration
    srv := &http.Server{
        Handler:      r,
        Addr:         "0.0.0.0:8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    fmt.Println("Server started at http://localhost:8080")
    log.Fatal(srv.ListenAndServe())
}

// WebSocket Handler
func wsHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("WebSocket upgrade error:", err)
        return
    }

    // Authentication
    cookie, err := r.Cookie("session_id")
    if err != nil {
        log.Println("No session_id cookie found")
        conn.Close()
        return
    }

    var user User
    err = db.QueryRow("SELECT id, nickname FROM users WHERE id = ?", cookie.Value).Scan(&user.ID, &user.Nickname)
    if err != nil {
        log.Printf("Failed to authenticate user with session_id %s: %v", cookie.Value, err)
        conn.Close()
        return
    }

    client := &Client{conn: conn, userId: user.ID}

    // Add client
    clientsMutex.Lock()
    clients[client] = true
    clientsMutex.Unlock()

    // Update status
    _, err = db.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", user.ID)
    if err != nil {
        log.Printf("Failed to set user %s online: %v", user.ID, err)
    }
    broadcastUserStatus(user.ID, true)
    broadcastOnlineUsers()

    defer func() {
        // Cleanup on disconnect
        clientsMutex.Lock()
        delete(clients, client)
        clientsMutex.Unlock()
        conn.Close()

        _, err := db.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", user.ID)
        if err != nil {
            log.Printf("Failed to set user %s offline: %v", user.ID, err)
        }
        broadcastUserStatus(user.ID, false)
        broadcastOnlineUsers()
    }()

    // Listen for messages
    for {
        var msg struct {
            Type    string          `json:"type"`
            Payload json.RawMessage `json:"payload"`
        }

        if err := conn.ReadJSON(&msg); err != nil {
            log.Printf("WebSocket read error for user %s: %v", user.ID, err)
            break
        }

        switch msg.Type {
        case "private_message":
            var message struct {
                ReceiverId string `json:"receiverId"`
                Content    string `json:"content"`
                MessageId  string `json:"messageId"` // Client-generated ID
            }
            if err := json.Unmarshal(msg.Payload, &message); err != nil {
                log.Printf("Failed to unmarshal private message: %v", err)
                continue
            }
            handlePrivateMessage(client, user.ID, message.ReceiverId, message.Content, message.MessageId)
        case "mark_read":
            var readData struct {
                SenderId  string `json:"senderId"`
                MessageId string `json:"messageId"`
            }
            if err := json.Unmarshal(msg.Payload, &readData); err != nil {
                log.Printf("Failed to unmarshal mark_read message: %v", err)
                continue
            }
            handleMarkRead(user.ID, readData.SenderId, readData.MessageId)
        }
    }
}

// WebSocket Functions
func broadcastUserStatus(userId string, isOnline bool) {
    message := map[string]interface{}{
        "type":     "user_status",
        "userId":   userId,
        "isOnline": isOnline,
    }

    clientsMutex.Lock()
    defer clientsMutex.Unlock()

    for client := range clients {
        if err := client.conn.WriteJSON(message); err != nil {
            log.Printf("Failed to broadcast user status to client %s: %v", client.userId, err)
            client.conn.Close()
            delete(clients, client)
        }
    }
}

func broadcastOnlineUsers() {
    rows, err := db.Query("SELECT id, nickname FROM users WHERE is_online = TRUE")
    if err != nil {
        log.Println("Failed to get online users:", err)
        return
    }
    defer rows.Close()

    var onlineUsers []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Nickname); err != nil {
            log.Printf("Failed to scan user: %v", err)
            continue
        }
        onlineUsers = append(onlineUsers, user)
    }

    message := map[string]interface{}{
        "type":    "online_users",
        "payload": onlineUsers,
    }

    clientsMutex.Lock()
    defer clientsMutex.Unlock()

    for client := range clients {
        if err := client.conn.WriteJSON(message); err != nil {
            log.Printf("Failed to broadcast online users to client %s: %v", client.userId, err)
            client.conn.Close()
            delete(clients, client)
        }
    }
}

func handlePrivateMessage(client *Client, senderId, receiverId, content, clientMessageId string) {
    messageId := uuid.New().String()
    _, err := db.Exec(`
        INSERT INTO private_messages (id, sender_id, receiver_id, content, is_read)
        VALUES (?, ?, ?, ?, ?)`,
        messageId, senderId, receiverId, content, false)
    if err != nil {
        log.Println("Failed to save message:", err)
        return
    }

    var senderNickname string
    err = db.QueryRow("SELECT nickname FROM users WHERE id = ?", senderId).Scan(&senderNickname)
    if err != nil {
        log.Printf("Failed to get sender nickname for user %s: %v", senderId, err)
        return
    }

    message := map[string]interface{}{
        "type": "private_message",
        "payload": map[string]interface{}{
            "messageId":      messageId,
            "clientMessageId": clientMessageId,
            "senderId":       senderId,
            "senderName":     senderNickname,
            "content":        content,
            "timestamp":      time.Now().Format(time.RFC3339),
            "isRead":         false,
        },
    }

    confirmation := map[string]interface{}{
        "type": "message_confirmation",
        "payload": map[string]interface{}{
            "messageId":      messageId,
            "clientMessageId": clientMessageId,
            "senderId":       senderId,
            "senderName":     senderNickname,
            "receiverId":     receiverId,
            "content":        content,
            "timestamp":      time.Now().Format(time.RFC3339),
            "isRead":         false,
        },
    }

    clientsMutex.Lock()
    defer clientsMutex.Unlock()

    // Send to receiver
    for c := range clients {
        if c.userId == receiverId {
            if err := c.conn.WriteJSON(message); err != nil {
                log.Printf("Failed to send message to client %s: %v", c.userId, err)
                c.conn.Close()
                delete(clients, c)
            }
        }
    }

    // Send confirmation to sender
    if err := client.conn.WriteJSON(confirmation); err != nil {
        log.Printf("Failed to send confirmation to sender %s: %v", senderId, err)
        client.conn.Close()
        delete(clients, client)
    }
}

func handleMarkRead(receiverId, senderId, messageId string) {
    _, err := db.Exec(`
        UPDATE private_messages SET is_read = TRUE
        WHERE id = ? AND sender_id = ? AND receiver_id = ? AND is_read = FALSE`,
        messageId, senderId, receiverId)
    if err != nil {
        log.Printf("Failed to mark message %s as read: %v", messageId, err)
        return
    }

    readMessage := map[string]interface{}{
        "type": "message_read",
        "payload": map[string]interface{}{
            "messageId":  messageId,
            "senderId":   senderId,
            "receiverId": receiverId,
        },
    }

    clientsMutex.Lock()
    defer clientsMutex.Unlock()

    // Notify sender
    for client := range clients {
        if client.userId == senderId {
            if err := client.conn.WriteJSON(readMessage); err != nil {
                log.Printf("Failed to notify sender %s of read status: %v", senderId, err)
                client.conn.Close()
                delete(clients, client)
            }
        }
    }
}

// Database Functions
func createTables() error {
    tables := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            nickname TEXT UNIQUE,
            age INTEGER,
            gender TEXT,
            first_name TEXT,
            last_name TEXT,
            email TEXT UNIQUE,
            password TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
            is_online BOOLEAN DEFAULT FALSE
        )`,
        `CREATE TABLE IF NOT EXISTS posts (
            id TEXT PRIMARY KEY,
            user_id TEXT,
            category TEXT,
            title TEXT,
            content TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(user_id) REFERENCES users(id)
        )`,
        `CREATE TABLE IF NOT EXISTS comments (
            id TEXT PRIMARY KEY,
            post_id TEXT,
            user_id TEXT,
            content TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(post_id) REFERENCES posts(id),
            FOREIGN KEY(user_id) REFERENCES users(id)
        )`,
        `CREATE TABLE IF NOT EXISTS private_messages (
            id TEXT PRIMARY KEY,
            sender_id TEXT,
            receiver_id TEXT,
            content TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            is_read BOOLEAN DEFAULT FALSE,
            FOREIGN KEY(sender_id) REFERENCES users(id),
            FOREIGN KEY(receiver_id) REFERENCES users(id)
        )`,
    }

    for _, table := range tables {
        _, err := db.Exec(table)
        if err != nil {
            return fmt.Errorf("failed to create table: %v", err)
        }
    }
    return nil
}

// API Handlers
func registerHandler(w http.ResponseWriter, r *http.Request) {
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
    _, err = db.Exec(`
        INSERT INTO users (id, nickname, age, gender, first_name, last_name, email, password) 
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        id, req.Nickname, req.Age, req.Gender, req.FirstName, req.LastName, req.Email, string(hashedPassword))
    if err != nil {
        if isDuplicateKeyError(err) {
            respondWithError(w, http.StatusConflict, "Nickname or email already exists")
            return
        }
        respondWithError(w, http.StatusInternalServerError, "Failed to create user")
        return
    }

    respondWithJSON(w, http.StatusCreated, map[string]string{"message": "User created successfully"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
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
    err := db.QueryRow(`
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

    _, err = db.Exec(`
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
        "user":    User{ID: user.ID, Nickname: user.Nickname},
    })
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    cookie, err := r.Cookie("session_id")
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Not logged in")
        return
    }

    _, err = db.Exec(`
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

func getCurrentUserHandler(w http.ResponseWriter, r *http.Request) {
    userID, err := authenticateUser(r)
    if err != nil {
        respondWithError(w, http.StatusUnauthorized, "Authentication required")
        return
    }

    var user User
    err = db.QueryRow(`
        SELECT id, nickname FROM users WHERE id = ?`,
        userID).Scan(&user.ID, &user.Nickname)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to get user info")
        return
    }

    respondWithJSON(w, http.StatusOK, user)
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
    rows, err := db.Query("SELECT id, nickname, is_online FROM users")
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to fetch users")
        return
    }
    defer rows.Close()

    var users []struct {
        User
        IsOnline bool `json:"isOnline"`
    }
    for rows.Next() {
        var user struct {
            User
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

func getPostsHandler(w http.ResponseWriter, r *http.Request) {
    rows, err := db.Query(`
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

func getPostHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    postID := vars["id"]

    var post struct {
        ID        string    `json:"id"`
        Title     string    `json:"title"`
        Content   string    `json:"content"`
        Category  string    `json:"category"`
        CreatedAt time.Time `json:"created_at"`
        Author    string    `json:"author"`
    }
    err := db.QueryRow(`
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

func createPostHandler(w http.ResponseWriter, r *http.Request) {
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
    _, err = db.Exec(`
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

func getCommentsHandler(w http.ResponseWriter, r *http.Request) {
    postID := r.URL.Query().Get("post_id")
    if postID == "" {
        respondWithError(w, http.StatusBadRequest, "Post ID required")
        return
    }

    rows, err := db.Query(`
        SELECT c.id, c.content, c.created_at, u.nickname 
        FROM comments c
        JOIN users u ON c.user_id = u.id
        WHERE c.post_id = ?
        ORDER BY c.created_at ASC`, postID)
    if err != nil {
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
            respondWithError(w, http.StatusInternalServerError, "Failed to process comments")
            return
        }
        comments = append(comments, comment)
    }

    respondWithJSON(w, http.StatusOK, comments)
}

func createCommentHandler(w http.ResponseWriter, r *http.Request) {
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

    commentID := uuid.New().String()
    _, err = db.Exec(`
        INSERT INTO comments (id, post_id, user_id, content) 
        VALUES (?, ?, ?, ?)`,
        commentID, req.PostID, userID, req.Content)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Failed to create comment")
        return
    }

    respondWithJSON(w, http.StatusCreated, map[string]string{
        "message":    "Comment created successfully",
        "comment_id": commentID,
    })
}

func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
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

    rows, err := db.Query(`
        SELECT m.id, m.sender_id, m.content, m.created_at, u.nickname, m.is_read
        FROM private_messages m
        JOIN users u ON m.sender_id = u.id
        WHERE (m.sender_id = ? AND m.receiver_id = ?) OR (m.sender_id = ? AND m.receiver_id = ?)
        ORDER BY m.created_at ASC`,
        userID, withUserId, withUserId, userID)
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

    respondWithJSON(w, http.StatusOK, messages)
}

// Utility Functions
func authenticateUser(r *http.Request) (string, error) {
    cookie, err := r.Cookie("session_id")
    if err != nil {
        return "", err
    }

    var userID string
    err = db.QueryRow("SELECT id FROM users WHERE id = ?", cookie.Value).Scan(&userID)
    if err != nil {
        return "", err
    }

    return userID, nil
}

func respondWithError(w http.ResponseWriter, code int, message string) {
    respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(payload)
}

func isDuplicateKeyError(err error) bool {
    if err == nil {
        return false
    }
    return err.Error() == "UNIQUE constraint failed: users.nickname" ||
        err.Error() == "UNIQUE constraint failed: users.email"
}