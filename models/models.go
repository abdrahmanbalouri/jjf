package models

import (
	"regexp"

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

func Skip(str string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	skip := re.ReplaceAllString(str, "")
	content := re.ReplaceAllString(skip, "")
    return   content
}
