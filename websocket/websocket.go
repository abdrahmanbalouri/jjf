package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

            "jj/api"
	"jj/database"
	"jj/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Global WebSocket-related variables managed within this package
var (
	Clients      = make(map[*models.Client]bool) // Use models.Client
  	ClientsMutex sync.Mutex
	Upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

// WsHandler manages WebSocket connections.
func WsHandler(w http.ResponseWriter, r *http.Request) {

	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	// Authentication
	 ids,err:= authenticateUser(r)
	 if err != nil {
		api.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var user models.User
	err = database.DB.QueryRow("SELECT id, nickname FROM users WHERE id = ?", ids).Scan(&user.ID, &user.Nickname)
	if err != nil {
		api.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		conn.Close()
		return
	}

	client := &models.Client{Conn: conn, UserID: user.ID}

	// Add client
	ClientsMutex.Lock()
	Clients[client] = true
	ClientsMutex.Unlock()

	// Update online status for all users
	allUsers := []string{}
	rows, err := database.DB.Query("SELECT id FROM users")
	if err != nil {
		log.Println("Failed to get users:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			log.Println("Failed to scan user ID:", err)
			continue
		}
		allUsers = append(allUsers, userID)
	}

	for _, r := range allUsers {
		kk := false
		ClientsMutex.Lock()
		for clien := range Clients {
			if clien.UserID == r {
				kk = true
				_, err := database.DB.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", r)
				if err != nil {
					log.Printf("Failed to set user %s online: %v", r, err)
				}
				break
			}
		}
		ClientsMutex.Unlock()
		if !kk {
			_, err := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", r)
			if err != nil {
				log.Printf("Failed to set user %s offline: %v", r, err)
			}
		}
	}

	// Update status
	_, err = database.DB.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", user.ID)
	if err != nil {
		log.Printf("Failed to set user %s online: %v", user.ID, err)
	}
	BroadcastOnlineUsers()

	defer func() {
		fmt.Println("22222")
		ClientsMutex.Lock()
		delete(Clients, client)
		ClientsMutex.Unlock()
		conn.Close()

		_, err := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", user.ID)
		if err != nil {
			log.Printf("Failed to set user %s offline: %v", user.ID, err)
		}
		for _, r := range allUsers {
			kk := false
			ClientsMutex.Lock()
			for clien := range Clients {
				if clien.UserID == r {
					kk = true
					_, err := database.DB.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", r)
					if err != nil {
						log.Printf("Failed to set user %s online: %v", r, err)
					}
					break
				}
			}
			ClientsMutex.Unlock()
			if !kk {
				_, err := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", r)
				if err != nil {
					log.Printf("Failed to set user %s offline: %v", r, err)
				}
			}
		}
		
		if(api.Lougout){
			_, err2 := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", user.ID)
			fmt.Println("mmm")
			if err2 != nil {
				
				
			}

			for c := range Clients {
		  if c.UserID == user.ID {
		      fmt.Println("22")
				c.Conn.Close()
				delete(Clients, c)
			
		}
	}

			
		}
		BroadcastOnlineUsers()
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
		fmt.Println(msg, "---------")
		switch msg.Type {
		case "private_message":
			var message struct {
				ReceiverID string `json:"receiverId"`
				Content    string `json:"content"`
				MessageID  string `json:"messageId"`
			}
			if err := json.Unmarshal(msg.Payload, &message); err != nil {
				log.Printf("Failed to unmarshal private message: %v", err)
				continue
			}
			HandlePrivateMessage(client, user.ID, message.ReceiverID, message.Content, message.MessageID)
		case "mark_read":
			var readData struct {
				SenderID  string `json:"senderId"`
				MessageID string `json:"messageId"`
			}
			if err := json.Unmarshal(msg.Payload, &readData); err != nil {
				log.Printf("Failed to unmarshal mark_read message: %v", err)
				continue
			}
			HandleMarkRead(user.ID, readData.SenderID, readData.MessageID)
		case "typing":
			var typingData struct {
				ReceiverID string `json:"receiverId"`
			}
			if err := json.Unmarshal(msg.Payload, &typingData); err != nil {
				log.Printf("Failed to unmarshal typing message: %v", err)
				continue
			}
			HandleTyping(client, user.ID, user.Nickname, typingData.ReceiverID)
		case "stop_typing":
			var typingData struct {
				ReceiverID string `json:"receiverId"`
			}
			if err := json.Unmarshal(msg.Payload, &typingData); err != nil {
				log.Printf("Failed to unmarshal stop_typing message: %v", err)
				continue
			}
			HandleStopTyping(client, user.ID, user.Nickname, typingData.ReceiverID)

		}
	}
}

// BroadcastUserStatus sends user online/offline status to all connected clients.
func BroadcastUserStatus(userID string, isOnline bool) {
	message := map[string]interface{}{
		"type":     "user_status",
		"userId":   userID,
		"isOnline": isOnline,
	}

	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	for client := range Clients {
		if err := client.Conn.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast user status to client %s: %v", client.UserID, err)
			client.Conn.Close()
			delete(Clients, client)
		}
	}
}

// BroadcastOnlineUsers sends a list of all currently online users to all connected clients.
func BroadcastOnlineUsers() {
	fmt.Println("5555")
	rows, err := database.DB.Query("SELECT id, nickname FROM users WHERE is_online = TRUE")
	if err != nil {
		log.Println("Failed to get online users:", err)
		return
	}
	defer rows.Close()

	var onlineUsers []models.User
	for rows.Next() {
		var user models.User
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

	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	for client := range Clients {
		if err := client.Conn.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast online users to client %s: %v", client.UserID, err)
			client.Conn.Close()
			delete(Clients, client)
		}
	}
}

// HandlePrivateMessage processes a private message from one user to another.
func HandlePrivateMessage(client *models.Client, senderID, receiverID, content, clientMessageID string) {
	messageID := uuid.New().String()
	eroor := map[string]interface{}{
		"type": "eroor",
		"payload": map[string]interface{}{
			"eroor": "try  a better message",
		},
	}
	if len(content) > 30 || content == "" {
		client.Conn.WriteJSON(eroor)
		return
	}
	Contentformessage := models.Skip(content)
	_, err := database.DB.Exec(`
        INSERT INTO private_messages (id, sender_id, receiver_id, content, is_read)
        VALUES (?, ?, ?, ?, ?)`,
		messageID, senderID, receiverID, Contentformessage, false)
	if err != nil {
		log.Println("Failed to save message:", err)
		return
	}

	var senderNickname string
	err = database.DB.QueryRow("SELECT nickname FROM users WHERE id = ?", senderID).Scan(&senderNickname)
	if err != nil {
		log.Printf("Failed to get sender nickname for user %s: %v", senderID, err)
		return
	}

	message := map[string]interface{}{
		"type": "private_message",
		"payload": map[string]interface{}{
			"messageId":       messageID,
			"clientMessageId": clientMessageID,
			"senderId":        senderID,
			"senderName":      senderNickname,
			"receiverId":      receiverID,
			"content":         Contentformessage,
			"timestamp":       time.Now().Format(time.RFC3339),
			"isRead":          false,
		},
	}

	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	// Send to all connections of the receiver
	for c := range Clients {
		if c.UserID == receiverID {
			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send message to client %s: %v", c.UserID, err)
				c.Conn.Close()
				delete(Clients, c)
			}
		}
	}

	// Send to all connections of the sender (including all their tabs)
	for c := range Clients {
		if c.UserID == senderID {
			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send message to sender %s: %v", senderID, err)
				c.Conn.Close()
				delete(Clients, c)
			}
		}
	}
}

// HandleMarkRead marks a message as read in the database and notifies the sender.
func HandleMarkRead(receiverID, senderID, messageID string) {
	_, err := database.DB.Exec(`
        UPDATE private_messages SET is_read = TRUE
        WHERE id = ? AND sender_id = ? AND receiver_id = ? AND is_read = FALSE`,
		messageID, senderID, receiverID)
	if err != nil {
		log.Printf("Failed to mark message %s as read: %v", messageID, err)
		return
	}

	readMessage := map[string]interface{}{
		"type": "message_read",
		"payload": map[string]interface{}{
			"messageId":  messageID,
			"senderId":   senderID,
			"receiverId": receiverID,
		},
	}

	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	// Notify sender
	for client := range Clients {
		if client.UserID == senderID {
			if err := client.Conn.WriteJSON(readMessage); err != nil {
				log.Printf("Failed to notify sender %s of read status: %v", senderID, err)
				client.Conn.Close()
				delete(Clients, client)
			}
		}
	}
}

// HandleTyping sends a typing event to the receiver.
func HandleTyping(client *models.Client, senderID, senderNickname, receiverID string) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	for c := range Clients {
		if c.UserID == receiverID {
			err := c.Conn.WriteJSON(struct {
				Type    string `json:"type"`
				Payload struct {
					SenderID   string `json:"senderId"`
					SenderName string `json:"senderName"`
				} `json:"payload"`
			}{
				Type: "typing",
				Payload: struct {
					SenderID   string `json:"senderId"`
					SenderName string `json:"senderName"`
				}{
					SenderID:   senderID,
					SenderName: senderNickname,
				},
			})
			if err != nil {
				log.Printf("Failed to send typing event to user %s: %v", receiverID, err)
				c.Conn.Close()
				delete(Clients, c)
			}
		}
	}
}

// HandleStopTyping sends a stop typing event to the receiver.
func HandleStopTyping(client *models.Client, senderID, senderNickname, receiverID string) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	for c := range Clients {
		if c.UserID == receiverID {
			err := c.Conn.WriteJSON(struct {
				Type    string `json:"type"`
				Payload struct {
					SenderID   string `json:"senderId"`
					SenderName string `json:"senderName"`
				} `json:"payload"`
			}{
				Type: "stop_typing",
				Payload: struct {
					SenderID   string `json:"senderId"`
					SenderName string `json:"senderName"`
				}{
					SenderID:   senderID,
					SenderName: senderNickname,
				},
			})
			if err != nil {
				log.Printf("Failed to send stop_typing event to user %s: %v", receiverID, err)
				c.Conn.Close()
				delete(Clients, c)
			}
		}
	}
}

func authenticateUser(r *http.Request) (string, error) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return "", err
	}
	fmt.Println(cookie)

	var userID string
	err = database.DB.QueryRow("SELECT id FROM users WHERE token = ?", cookie.Value).Scan(&userID)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return userID, nil
}