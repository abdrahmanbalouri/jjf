package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"jj/database" // Import the database package
	"jj/models"   // Import your models

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
	conn, err := Upgrader.Upgrade(w, r, nil) // Use package-level Upgrader
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
	// for client := range Clients{

	// }

	var user models.User // Use models.User
	err = database.DB.QueryRow("SELECT id, nickname FROM users WHERE id = ?", cookie.Value).Scan(&user.ID, &user.Nickname)
	if err != nil {
		log.Printf("Failed to authenticate user with session_id %s: %v", cookie.Value, err)
		conn.Close()
		return
	}

	client := &models.Client{Conn: conn, UserID: user.ID} // Use models.Client

	// Add client
	ClientsMutex.Lock()    // Use package-level mutex
	Clients[client] = true // Use package-level clients map
	ClientsMutex.Unlock()
	allUsers := []string{}
	rows, err := database.DB.Query("SELECT id FROM users")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return
		}
		allUsers = append(allUsers, userID)
	}

	for _,r := range allUsers {
		kk := false
		for  clien := range Clients {
			if clien.UserID != r {
				continue
			} else {
                fmt.Println("11")
				kk = true
				_, err := database.DB.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", r)
				if err != nil {
				}
				break
			}
		}
		if !kk {
            fmt.Println("22")
			_, err := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", r)
			if err != nil {
			}

		}

	}


	// Update status
	_, err = database.DB.Exec("UPDATE users SET is_online = TRUE WHERE id = ?", user.ID)
	if err != nil {
		log.Printf("Failed to set user %s online: %v", user.ID, err)
	}
	BroadcastUserStatus(user.ID, true) // Call public function
	BroadcastOnlineUsers()             // Call public function

	defer func() {
		ClientsMutex.Lock()
		delete(Clients, client)
		ClientsMutex.Unlock()
		conn.Close()

		_, err := database.DB.Exec("UPDATE users SET is_online = FALSE WHERE id = ?", user.ID)
		if err != nil {
			log.Printf("Failed to set user %s offline: %v", user.ID, err)
		}
		BroadcastUserStatus(user.ID, false)
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
		fmt.Println(msg)

		switch msg.Type {
		case "private_message":
			var message struct {
				ReceiverID string `json:"receiverId"`
				Content    string `json:"content"`
				MessageID  string `json:"messageId"` // Client-generated ID
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
	rows, err := database.DB.Query("SELECT id, nickname FROM users WHERE is_online = TRUE")
	if err != nil {
		log.Println("Failed to get online users:", err)
		return
	}
	defer rows.Close()

	var onlineUsers []models.User // Use models.User
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
	_, err := database.DB.Exec(`
        INSERT INTO private_messages (id, sender_id, receiver_id, content, is_read)
        VALUES (?, ?, ?, ?, ?)`,
		messageID, senderID, receiverID, content, false)
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
			"content":         content,
			"timestamp":       time.Now().Format(time.RFC3339),
			"isRead":          false,
		},
	}

	confirmation := map[string]interface{}{
		"type": "private_message",
		"payload": map[string]interface{}{
			"messageId":       messageID,
			"clientMessageId": clientMessageID,
			"senderId":        senderID,
			"senderName":      senderNickname,
			"receiverId":      receiverID,
			"content":         content,
			"timestamp":       time.Now().Format(time.RFC3339),
			"isRead":          false,
		},
	}

	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	// Send to receiver
	for c := range Clients {
		if c.UserID == receiverID {
			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("Failed to send message to client %s: %v", c.UserID, err)
				c.Conn.Close()
				delete(Clients, c)
			}
		}
	}

	// Send confirmation to sender
	if err := client.Conn.WriteJSON(confirmation); err != nil {
		log.Printf("Failed to send confirmation to sender %s: %v", senderID, err)
		client.Conn.Close()
		delete(Clients, client)
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
			}
		}
	}
}
