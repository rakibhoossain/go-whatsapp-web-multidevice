package websocket

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type Client struct {
	conn     *websocket.Conn
	username string
}

type BroadcastMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  any    `json:"result"`
}

var (
	// Map of channel names to connected clients with mutex protection
	channels = struct {
		sync.RWMutex
		m map[string]map[*Client]bool
	}{m: make(map[string]map[*Client]bool)}

	// Channels for hub operations
	Register  = make(chan *Client)
	Broadcast = make(chan struct {
		Channel string
		Message BroadcastMessage
	})
	Unregister = make(chan *Client)
)

func handleRegister(client *Client) {
	channels.Lock()
	defer channels.Unlock()

	if _, exists := channels.m[client.username]; !exists {
		channels.m[client.username] = make(map[*Client]bool)
	}
	channels.m[client.username][client] = true
	log.Printf("client registered to channel: %s (total clients: %d)", client.username, len(channels.m[client.username]))
}

func handleUnregister(client *Client) {
	channels.Lock()
	defer channels.Unlock()

	if clients, exists := channels.m[client.username]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(channels.m, client.username)
		}
		log.Printf("client unregistered from channel: %s (remaining clients: %d)", client.username, len(clients))
	}
}

func broadcastMessage(channel string, message BroadcastMessage) {
	marshalMessage, err := json.Marshal(message)
	if err != nil {
		log.Println("marshal error:", err)
		return
	}

	channels.RLock()
	defer channels.RUnlock()

	if clients, exists := channels.m[channel]; exists {
		for client := range clients {
			if err := client.conn.WriteMessage(websocket.TextMessage, marshalMessage); err != nil {
				log.Println("write error:", err)
				go closeConnection(client) // Handle in goroutine to avoid deadlock
			}
		}
	}
}

func closeConnection(client *Client) {
	// Send close message and clean up
	if err := client.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
		log.Println("write close message error:", err)
	}
	if err := client.conn.Close(); err != nil {
		log.Println("close connection error:", err)
	}
	Unregister <- client
}

func RunHub() {
	for {
		select {
		case client := <-Register:
			handleRegister(client)

		case client := <-Unregister:
			handleUnregister(client)

		case msg := <-Broadcast:
			broadcastMessage(msg.Channel, msg.Message)
		}
	}
}

// authenticateUser checks credentials against PostgreSQL
func authenticateUser(db *sql.DB, token, password string) (bool, error) {

	fmt.Println(token, password)

	var hashedPassword string
	err := db.QueryRow("SELECT token FROM whatsmeow_device_client_pivot WHERE token = $1", token).Scan(&hashedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // User not found
		}
		return false, err // Database error
	}

	return len(hashedPassword) > 2, nil
}

func RegisterRoutes(app *fiber.App, db *sql.DB) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.SendStatus(fiber.StatusUpgradeRequired)
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {

		// Get token from query parameters
		authHeader := conn.Query("token")
		if authHeader == "" {
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Token required"))
			conn.Close()
			return
		}

		payload, err := base64.StdEncoding.DecodeString(authHeader)

		if err != nil {
			conn.WriteMessage(websocket.CloseMessage, []byte("400 Bad Request"))
			conn.Close()
			return
		}

		creds := strings.SplitN(string(payload), ":", 2)
		if len(creds) != 2 {
			conn.WriteMessage(websocket.CloseMessage, []byte("400 Bad Request"))
			conn.Close()
			return
		}

		username, password := creds[0], creds[1]

		// Authenticate against PostgreSQL
		authenticated, err := authenticateUser(db, username, password)
		if err != nil {
			log.Println("authentication error:", err)
			conn.WriteMessage(websocket.CloseMessage, []byte("500 Internal Server Error"))
			conn.Close()
			return
		}

		if !authenticated {
			conn.WriteMessage(websocket.CloseMessage, []byte("401 Unauthorized"))
			conn.Close()
			return
		}

		// Create client and register to their channel (username)
		client := &Client{
			conn:     conn,
			username: username,
		}

		defer func() {
			Unregister <- client
			conn.Close()
		}()

		Register <- client

		for {
			messageType, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}
				return
			}

			if messageType != websocket.TextMessage {
				log.Println("unsupported message type:", messageType)
				continue
			}

			// Client can only listen to their own channel (username)
			// No action needed here unless you want to handle specific client messages
		}
	}))
}

// GetChannelClientsCount returns the number of clients connected to a channel
func GetChannelClientsCount(channel string) int {
	channels.RLock()
	defer channels.RUnlock()

	if clients, exists := channels.m[channel]; exists {
		return len(clients)
	}
	return 0
}
