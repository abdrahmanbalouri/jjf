package models

import (
	"github.com/gorilla/websocket"
)

// User represents a forum user.
type User struct {
    ID       string `json:"id"`
    Nickname string `json:"nickname"`
}

// Client represents a connected WebSocket client.
type Client struct {
    Conn   *websocket.Conn
    UserID string
}