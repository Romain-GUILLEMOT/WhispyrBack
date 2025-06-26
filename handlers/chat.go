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

// La SEULE et UNIQUE définition de la struct Message, avec le nom du serveur.
type Message struct {
	Type       string `json:"type"`
	ServerID   string `json:"serverId,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	UserID     string `json:"userId,omitempty"`
	Username   string `json:"username,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	Content    string `json:"content,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
	Status     string `json:"status,omitempty"`
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
	// Récupération de l'ID utilisateur
	userIDPtr, ok := c.Locals("user_id").(*uuid.UUID)
	if !ok || userIDPtr == nil || *userIDPtr == uuid.Nil {
		utils.Error("userID non trouvé ou invalide dans les locaux du contexte Fiber.")
		c.Close()
		return
	}
	userId := *userIDPtr

	// Récupération des données utilisateur
	userData, err := dbTools.GetUserByID(&userId)
	if err != nil {
		utils.Error("Erreur lors de la récupération des données utilisateur (" + userId.String() + "): " + err.Error())
		c.Close()
		return
	}

	// Création du client WebSocket
	currentClient := &Client{
		UserID:   userId,
		Username: userData.Username,
		Avatar:   userData.Avatar,
		Conn:     c,
	}

	mutex.Lock()
	clients[c] = currentClient
	mutex.Unlock()

	// Gestion de la présence (online)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := utils.RedisSAdd(ctx, "online_users", currentClient.UserID.String()); err != nil {
			utils.Error("Erreur RedisSAdd online_users pour " + currentClient.UserID.String() + ": " + err.Error())
		}
		presenceOnlineMsg, _ := json.Marshal(Message{
			Type: "presence", UserID: currentClient.UserID.String(), Username: currentClient.Username,
			Avatar: currentClient.Avatar, Content: "s'est connecté", Status: "online", Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		})
		if err := utils.RedisPublish(ctx, "user:presence:updates", presenceOnlineMsg); err != nil {
			utils.Error("Erreur publication présence online: " + err.Error())
		}
	}()

	utils.Info("🎉 Utilisateur connecté: " + currentClient.Username + " (" + currentClient.UserID.String() + ")")

	// Gestion de la déconnexion
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
			Type: "presence", UserID: currentClient.UserID.String(), Username: currentClient.Username,
			Avatar: currentClient.Avatar, Content: "s'est déconnecté", Status: "offline", Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		})
		if err := utils.RedisPublish(ctx, "user:presence:updates", presenceOfflineMsg); err != nil {
			utils.Error("Erreur publication présence offline: " + err.Error())
		}
	}()

	// Boucle de lecture des messages
	for {
		_, rawMsg, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err) {
				utils.Info("Connexion fermée inopinément par le client: " + err.Error())
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
			if incomingMessage.ServerID != "" && incomingMessage.ServerID == currentClient.CurrentServerID {
				chatMsg := Message{
					Type: "chat", ServerID: incomingMessage.ServerID, UserID: currentClient.UserID.String(),
					Username: currentClient.Username, Avatar: currentClient.Avatar, Content: incomingMessage.Content,
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
				}
				marshaledChatMsg, _ := json.Marshal(chatMsg)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				utils.RedisPublish(ctx, "chat:server:"+incomingMessage.ServerID, marshaledChatMsg)
				go dbTools.SaveMessageToScylla(context.Background(), chatMsg.ServerID, chatMsg.UserID, chatMsg.Content, chatMsg.Timestamp)
			} else {
				utils.Warn(fmt.Sprintf(
					"Message rejeté de %s. Tentative d'envoi au salon [%s] alors qu'il est dans le salon [%s].",
					currentClient.Username,
					incomingMessage.ServerID,
					currentClient.CurrentServerID,
				))
			}

		case "join_server":
			if incomingMessage.ServerID != "" {
				mutex.Lock()
				currentClient.CurrentServerID = incomingMessage.ServerID
				mutex.Unlock()

				utils.Info(fmt.Sprintf("Utilisateur %s a rejoint le salon [%s]", currentClient.Username, currentClient.CurrentServerID))

				server, err := dbTools.GetServerByID(incomingMessage.ServerID)
				if err != nil {
					utils.Error("Impossible de récupérer les détails du serveur " + incomingMessage.ServerID + ": " + err.Error())
					continue
				}

				confirmationMsg := Message{
					Type:       "join_server_success",
					ServerID:   server.ServerID.String(),
					ServerName: server.Name,
				}
				marshaledMsg, _ := json.Marshal(confirmationMsg)
				if err := c.WriteMessage(websocket.TextMessage, marshaledMsg); err != nil {
					utils.Error("Erreur envoi confirmation de join à " + currentClient.Username + ": " + err.Error())
				}
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

// StartBroadcaster reste inchangée, sa logique est bonne.
func StartBroadcaster() {
	ctx := context.Background()
	pubsub := utils.RedisPSubscribe(ctx, "chat:server:*", "user:presence:updates")
	if pubsub == nil {
		utils.Fatal("Broadcaster Redis - pubsub client est nil après PSubscribe.")
	}
	ch := pubsub.Channel()

	utils.Info("Broadcaster Redis démarré et abonné aux canaux.")
	utils.Info("Attente des messages Pub/Sub...")

	go func() {
		for msg := range ch {
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
		utils.Info("Broadcaster Redis: Le canal de messages a été fermé, goroutine arrêtée.")
	}()
}
