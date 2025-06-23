package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"sync"
)

type Client struct {
	UserID          uuid.UUID
	Username        string
	Avatar          string
	Conn            *websocket.Conn
	CurrentServerID string
}

type Message struct {
	Type      string `json:"type"`
	ServerID  string `json:"serverId,omitempty"`
	UserID    string `json:"userId,omitempty"`
	Username  string `json:"username,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
	Content   string `json:"content,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Status    string `json:"status,omitempty"`
}

var (
	clients = make(map[*websocket.Conn]*Client)
	mutex   = &sync.Mutex{}
)

func WebSocketHandler(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func HandleWebSocket(c *websocket.Conn) {
	userIDPtr, ok := c.Locals("user_id").(*uuid.UUID)
	utils.Info("TEST 2 : " + userIDPtr.String())

	if !ok || userIDPtr == nil || *userIDPtr == uuid.Nil {
		utils.Error("userID non trouvé ou invalide dans les locaux du contexte Fiber.")
		c.Close()
		return
	}
	userId := *userIDPtr

	userData, err := dbTools.GetUserByID(&userId)
	if err != nil {
		utils.Error("Erreur lors de la récupération des données utilisateur (" + userId.String() + "): " + err.Error())
		c.Close()
		return
	}

	currentClient := &Client{
		UserID:   userId,
		Username: userData.Username,
		Avatar:   userData.Avatar,
		Conn:     c,
	}

	mutex.Lock()
	clients[c] = currentClient
	mutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := utils.RedisSAdd(ctx, "online_users", currentClient.UserID.String()); err != nil {
		utils.Error("Erreur RedisSAdd online_users pour " + currentClient.UserID.String() + ": " + err.Error())
	}

	presenceOnlineMsg, _ := json.Marshal(Message{
		Type:      "presence",
		UserID:    currentClient.UserID.String(),
		Username:  currentClient.Username,
		Avatar:    currentClient.Avatar,
		Content:   "s'est connecté",
		Status:    "online",
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	})
	if err := utils.RedisPublish(ctx, "user:presence:updates", presenceOnlineMsg); err != nil {
		utils.Error("Erreur publication présence online: " + err.Error())
	}

	utils.Info("🎉 Utilisateur connecté: " + currentClient.Username + " (" + currentClient.UserID.String() + ")")

	defer func() {
		utils.Info("🧹 Déconnexion détectée pour: " + currentClient.Username)

		mutex.Lock()
		delete(clients, c)
		mutex.Unlock()

		c.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := utils.RedisSRem(ctx, "online_users", currentClient.UserID.String()); err != nil {
			utils.Error("Erreur SRem online_users pour " + currentClient.UserID.String() + ": " + err.Error())
		}

		presenceOfflineMsg, _ := json.Marshal(Message{
			Type:      "presence",
			UserID:    currentClient.UserID.String(),
			Username:  currentClient.Username,
			Avatar:    currentClient.Avatar,
			Content:   "s'est déconnecté",
			Status:    "offline",
			Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		})
		if err := utils.RedisPublish(ctx, "user:presence:updates", presenceOfflineMsg); err != nil {
			utils.Error("Erreur publication présence offline: " + err.Error())
		}
	}()

	for {
		_, rawMsg, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				utils.Info("Connexion fermée inopinément par le client: " + err.Error())
			} else {
				utils.Error("Erreur de lecture WebSocket pour " + currentClient.Username + ": " + err.Error())
			}
			break
		}

		var incomingMessage Message
		if err := json.Unmarshal(rawMsg, &incomingMessage); err != nil {
			utils.Error("Erreur unmarshalling message entrant JSON: " + err.Error())
			continue
		}

		switch incomingMessage.Type {
		case "chat":
			if incomingMessage.ServerID != "" {
				chatMsg := Message{
					Type:      "chat",
					ServerID:  incomingMessage.ServerID,
					UserID:    currentClient.UserID.String(),
					Username:  currentClient.Username,
					Avatar:    currentClient.Avatar,
					Content:   incomingMessage.Content,
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
				}

				marshaledChatMsg, _ := json.Marshal(chatMsg)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				utils.Info(fmt.Sprintf("DEBUG: Attempting to publish message to Redis channel: chat:server:%s", incomingMessage.ServerID))
				if err := utils.RedisPublish(ctx, "chat:server:"+incomingMessage.ServerID, marshaledChatMsg); err != nil {
					utils.Error("Erreur publication chat sur Redis: " + err.Error())
				} else {
					utils.Info("DEBUG: Message successfully published to Redis!")
				}

				go func(msg Message) {
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer saveCancel()
					if err := dbTools.SaveMessageToScylla(saveCtx, msg.ServerID, msg.UserID, msg.Content, msg.Timestamp); err != nil {
						utils.Error("Erreur lors de l'enregistrement du message dans ScyllaDB: " + err.Error())
					}
				}(chatMsg)
			} else {
				utils.Warn("Message de chat reçu sans ServerID.")
			}
		case "join_server":
			if incomingMessage.ServerID != "" {
				mutex.Lock()
				currentClient.CurrentServerID = incomingMessage.ServerID
				mutex.Unlock()
				utils.Info("Utilisateur " + currentClient.Username + " a changé pour le serveur: " + currentClient.CurrentServerID)
			} else {
				utils.Warn("Message 'join_server' reçu sans ServerID.")
			}
		case "heartbeat":
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := utils.RedisSetWithTTL(ctx, "user:last_seen:"+currentClient.UserID.String(), time.Now().Unix(), 30*time.Second); err != nil {
				utils.Error("Erreur mise à jour heartbeat: " + err.Error())
			}
		default:
			utils.Warn("Type de message inconnu reçu du client: " + incomingMessage.Type)
		}
	}
}

func StartBroadcaster() {
	ctx := context.Background() // Contexte de longue durée pour l'abonnement

	// Testez l'abonnement
	// pubsub, err := utils.Redis.PSubscribe(ctx, "chat:server:*", "user:presence:updates")
	// if err != nil {
	// 	utils.Fatal("Broadcaster Redis - Erreur lors de l'abonnement initial: " + err.Error())
	// }
	// defer pubsub.Close() // Fermer à la fin de StartBroadcaster

	// Pour s'assurer que l'abonnement se fait, et que le canal est prêt
	pubsub := utils.RedisPSubscribe(ctx, "chat:server:*", "user:presence:updates")
	if pubsub == nil {
		utils.Fatal("Broadcaster Redis - pubsub client est nil après PSubscribe.")
	}
	// Récupérer le canal de réception des messages
	ch := pubsub.Channel()

	utils.Info("Broadcaster Redis démarré et abonné aux canaux.")
	utils.Info("Attente des messages Pub/Sub...") // Log pour confirmer que nous attendons

	go func() {
		for msg := range ch { // Lisez depuis le canal des messages
			utils.Info("DEBUG: Broadcaster received message from Redis channel: " + msg.Channel)
			var event Message
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				utils.Error("Erreur unmarshalling message Redis (" + msg.Channel + "): " + err.Error())
				continue
			}

			mutex.Lock()
			for conn, client := range clients {
				if event.Type == "chat" && client.CurrentServerID == event.ServerID {
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
						utils.Error("Erreur envoi message chat à client (" + client.Username + "): " + err.Error())
						conn.Close()
						delete(clients, conn)
					}
				} else if event.Type == "presence" {
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
						utils.Error("Erreur envoi message présence à client (" + client.Username + "): " + err.Error())
						conn.Close()
						delete(clients, conn)
					}
				}
			}
			mutex.Unlock()
		}
		utils.Info("Broadcaster Redis: Le canal de messages a été fermé, goroutine arrêtée.") // Si la boucle se termine
	}()
}
