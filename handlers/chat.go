package handlers

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"sync"
)

type Client struct {
	Username string
	Avatar   string
	Conn     *websocket.Conn
}
type Message struct {
	Type     string `json:"type"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Content  string `json:"content"`
}

var (
	clients   = make(map[*websocket.Conn]*Client)
	broadcast = make(chan []byte)
	mutex     = &sync.Mutex{}
)

func WebSocketHandler(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func HandleWebSocket(c *websocket.Conn) {
	var currentUser *Client

	defer func() {
		utils.Info("🧹 Déconnexion détectée")
		utils.Info(currentUser.Avatar)
		mutex.Lock()
		if currentUser != nil {
			quitMsg, _ := json.Marshal(Message{
				Type:     "quit",
				Username: currentUser.Username,
				Avatar:   currentUser.Avatar,
				Content:  "a quitté le salon",
			})
			mutex.Unlock() // 🔓 pour éviter le deadlock pendant le broadcast
			broadcast <- quitMsg

			mutex.Lock()
			delete(clients, c)
		}
		mutex.Unlock()
		c.Close()
	}()

	userId, ok := c.Locals("user_id").(*uuid.UUID)
	if !ok || userId == nil {
		return
	}

	data, err := dbTools.GetUserByID(userId)
	if err != nil {
		utils.Error(err.Error())
		return
	}
	username := data.Username
	avatar := data.Avatar

	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			break
		}

		message := string(msg)

		if clients[c] == nil {
			currentUser = &Client{
				Username: username,
				Avatar:   avatar,
				Conn:     c,
			}
			clients[c] = currentUser

			joinMsg, _ := json.Marshal(Message{
				Type:     "join",
				Username: username,
				Avatar:   avatar,
				Content:  "a rejoint le salon",
			})
			broadcast <- joinMsg
			continue
		}

		chatMsg, _ := json.Marshal(Message{
			Type:     "message",
			Username: username,
			Avatar:   avatar,
			Content:  message,
		})
		broadcast <- chatMsg
	}
}

func StartBroadcaster() {
	go func() {
		for {
			msg := <-broadcast
			mutex.Lock()
			for conn := range clients {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					conn.Close()
					delete(clients, conn)
				}
			}
			mutex.Unlock()
		}
	}()
}
