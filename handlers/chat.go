package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools" // Assurez-vous que ce chemin est correct
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

type Client struct {
	UserID           uuid.UUID
	Username         string
	Avatar           string
	Conn             *websocket.Conn
	CurrentServerID  string // Le serveur actuellement "sélectionné" par le client (contexte principal)
	CurrentChannelID string // Le canal actuellement "actif" par le client pour la communication
}

type Message struct {
	Type        string `json:"type"`
	ServerID    string `json:"serverId,omitempty"`
	ChannelID   string `json:"channelId,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
	ServerName  string `json:"serverName,omitempty"`
	UserID      string `json:"userId,omitempty"`
	Username    string `json:"username,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	Content     string `json:"content,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	Status      string `json:"status,omitempty"`
	// RecipientID est retiré car les messages privés ne sont pas gérés pour le moment.
}

var (
	clients      = make(map[*websocket.Conn]*Client)
	clientsMutex = &sync.RWMutex{}
)

func WebSocketHandler(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

func HandleWebSocket(c *websocket.Conn) {
	userIDPtr, ok := c.Locals("user_id").(*uuid.UUID)
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

	clientsMutex.Lock()
	clients[c] = currentClient
	clientsMutex.Unlock()

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

	defer func() {
		utils.Info("🧹 Déconnexion détectée pour: " + currentClient.Username)
		clientsMutex.Lock()
		delete(clients, c)
		clientsMutex.Unlock()
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
			handleChatMessage(currentClient, incomingMessage)
		case "join_server":
			if err := handleJoinServer(c, currentClient, incomingMessage); err != nil {
				utils.Error("Erreur lors de la jointure du serveur pour " + currentClient.Username + ": " + err.Error())
			}
		case "leave_server":
			handleLeaveServer(c, currentClient, incomingMessage)
		case "join_channel":
			if err := handleJoinChannel(c, currentClient, incomingMessage); err != nil {
				utils.Error("Erreur lors de la jointure du canal pour " + currentClient.Username + ": " + err.Error())
			}
		case "leave_channel":
			handleLeaveChannel(c, currentClient, incomingMessage)
		case "heartbeat":
			if err := handleHeartbeat(currentClient); err != nil {
				utils.Error("Erreur heartbeat pour " + currentClient.Username + ": " + err.Error())
			}
		default:
			utils.Warn("Type de message inconnu reçu du client: " + incomingMessage.Type)
		}
	}
}

func handleJoinServer(c *websocket.Conn, currentClient *Client, incomingMessage Message) error {
	if incomingMessage.ServerID == "" {
		return fmt.Errorf("ServerID manquant pour la requête join_server")
	}

	clientsMutex.Lock()
	currentClient.CurrentServerID = incomingMessage.ServerID
	currentClient.CurrentChannelID = "" // Le client devra explicitement rejoindre un canal après
	clientsMutex.Unlock()

	utils.Info(fmt.Sprintf("Utilisateur %s a rejoint (sélectionné) le serveur [%s]", currentClient.Username, currentClient.CurrentServerID))

	server, err := dbTools.GetServerByID(incomingMessage.ServerID)
	if err != nil {
		return fmt.Errorf("impossible de récupérer les détails du serveur %s: %w", incomingMessage.ServerID, err)
	}

	confirmationMsg := Message{
		Type:       "join_server_success",
		ServerID:   server.ServerID.String(),
		ServerName: server.Name,
	}
	marshaledMsg, err := json.Marshal(confirmationMsg)
	if err != nil {
		return fmt.Errorf("erreur lors de l'encodage JSON du message de confirmation: %w", err)
	}

	if err := c.WriteMessage(websocket.TextMessage, marshaledMsg); err != nil {
		return fmt.Errorf("erreur envoi confirmation de join à %s: %w", currentClient.Username, err)
	}
	return nil
}

func handleLeaveServer(c *websocket.Conn, currentClient *Client, incomingMessage Message) {
	if currentClient.CurrentServerID == incomingMessage.ServerID {
		clientsMutex.Lock()
		currentClient.CurrentServerID = ""  // Réinitialise le serveur actuel
		currentClient.CurrentChannelID = "" // Réinitialise le canal aussi
		clientsMutex.Unlock()
		utils.Info(fmt.Sprintf("Utilisateur %s a quitté le serveur [%s]", currentClient.Username, incomingMessage.ServerID))

		leaveMsg, _ := json.Marshal(Message{
			Type: "presence", UserID: currentClient.UserID.String(), Username: currentClient.Username,
			Content: fmt.Sprintf("a quitté le serveur %s", incomingMessage.ServerID), Status: "left_server",
			ServerID: incomingMessage.ServerID, Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		utils.RedisPublish(ctx, "server:presence:updates:"+incomingMessage.ServerID, leaveMsg)

		confirmationMsg, _ := json.Marshal(Message{Type: "leave_server_success", ServerID: incomingMessage.ServerID})
		if err := c.WriteMessage(websocket.TextMessage, confirmationMsg); err != nil {
			utils.Error("Erreur envoi confirmation de leave à " + currentClient.Username + ": " + err.Error())
		}
	} else {
		utils.Warn(fmt.Sprintf("Utilisateur %s a tenté de quitter le serveur %s mais n'y est pas (actuellement dans %s)",
			currentClient.Username, incomingMessage.ServerID, currentClient.CurrentServerID))
	}
}

func handleJoinChannel(c *websocket.Conn, currentClient *Client, incomingMessage Message) error {
	if incomingMessage.ServerID == "" || incomingMessage.ChannelID == "" {
		return fmt.Errorf("ServerID ou ChannelID manquant pour la requête join_channel")
	}

	if currentClient.CurrentServerID != incomingMessage.ServerID {
		return fmt.Errorf("le client n'est pas actif dans le serveur %s pour rejoindre le canal %s", incomingMessage.ServerID, incomingMessage.ChannelID)
	}

	clientsMutex.Lock()
	currentClient.CurrentChannelID = incomingMessage.ChannelID
	clientsMutex.Unlock()

	utils.Info(fmt.Sprintf("Utilisateur %s a rejoint le canal [%s] du serveur [%s]", currentClient.Username, incomingMessage.ChannelID, incomingMessage.ServerID))

	channel, err := dbTools.GetChannelByID(incomingMessage.ChannelID)
	if err != nil {
		return fmt.Errorf("impossible de récupérer les détails du serveur %s: %w", incomingMessage.ChannelID, err)
	}

	confirmationMsg, err := json.Marshal(Message{Type: "join_channel_success", ServerID: incomingMessage.ServerID, ChannelID: incomingMessage.ChannelID, ChannelName: channel.Name})
	if err != nil {
		return fmt.Errorf("erreur lors de l'encodage JSON du message de confirmation: %w", err)
	}

	if err := c.WriteMessage(websocket.TextMessage, confirmationMsg); err != nil {
		return fmt.Errorf("erreur envoi confirmation de join channel à %s: %w", currentClient.Username, err)
	}
	return nil
}

func handleLeaveChannel(c *websocket.Conn, currentClient *Client, incomingMessage Message) {
	if currentClient.CurrentChannelID == incomingMessage.ChannelID {
		clientsMutex.Lock()
		currentClient.CurrentChannelID = "" // Réinitialise le canal actif
		clientsMutex.Unlock()
		utils.Info(fmt.Sprintf("Utilisateur %s a quitté le canal [%s] du serveur [%s]", currentClient.Username, incomingMessage.ChannelID, incomingMessage.ServerID))

		confirmationMsg, _ := json.Marshal(Message{Type: "leave_channel_success", ServerID: incomingMessage.ServerID, ChannelID: incomingMessage.ChannelID})
		if err := c.WriteMessage(websocket.TextMessage, confirmationMsg); err != nil {
			utils.Error("Erreur envoi confirmation de leave channel à " + currentClient.Username + ": " + err.Error())
		}
	} else {
		utils.Warn(fmt.Sprintf("Utilisateur %s a tenté de quitter le canal %s mais n'y est pas (actuellement dans %s)",
			currentClient.Username, incomingMessage.ChannelID, currentClient.CurrentChannelID))
	}
}

func handleChatMessage(currentClient *Client, incomingMessage Message) {
	if incomingMessage.ServerID == "" || incomingMessage.ChannelID == "" || incomingMessage.Content == "" {
		utils.Warn(fmt.Sprintf("Message de chat incomplet de %s.", currentClient.Username))
		return
	}

	if currentClient.CurrentServerID != incomingMessage.ServerID || currentClient.CurrentChannelID != incomingMessage.ChannelID {
		utils.Warn(fmt.Sprintf(
			"Message rejeté de %s. Le client n'est pas dans le serveur/canal spécifié. Client: S[%s]/C[%s], Msg: S[%s]/C[%s]",
			currentClient.Username,
			currentClient.CurrentServerID,
			currentClient.CurrentChannelID,
			incomingMessage.ServerID,
			incomingMessage.ChannelID,
		))
		return
	}

	chatMsg := Message{
		Type:      "chat",
		ServerID:  incomingMessage.ServerID,
		ChannelID: incomingMessage.ChannelID,
		UserID:    currentClient.UserID.String(),
		Username:  currentClient.Username,
		Avatar:    currentClient.Avatar,
		Content:   incomingMessage.Content,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	marshaledChatMsg, err := json.Marshal(chatMsg)
	if err != nil {
		utils.Error("Erreur encodage JSON du message de chat: " + err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := utils.RedisPublish(ctx, "chat:channel:"+incomingMessage.ChannelID, marshaledChatMsg); err != nil {
		utils.Error("Erreur publication message chat Redis: " + err.Error())
	}

	go dbTools.SaveMessageToScylla(context.Background(), chatMsg.ServerID, chatMsg.ChannelID, chatMsg.UserID, chatMsg.Content, chatMsg.Timestamp)
}

func handleHeartbeat(currentClient *Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := utils.RedisSetWithTTL(ctx, "user:last_seen:"+currentClient.UserID.String(), time.Now().Unix(), 30*time.Second); err != nil {
		return fmt.Errorf("erreur mise à jour heartbeat: %w", err)
	}
	return nil
}

func StartBroadcaster() {
	ctx := context.Background()
	// Suppression de l'abonnement aux messages privés
	pubsub := utils.RedisPSubscribe(ctx, "chat:channel:*", "user:presence:updates", "server:presence:updates:*")
	if pubsub == nil {
		utils.Fatal("Broadcaster Redis - pubsub client est nil après PSubscribe.")
	}
	ch := pubsub.Channel()

	utils.Info("Broadcaster Redis démarré et abonné aux canaux.")
	utils.Info("Attente des messages Pub/Sub...")

	go func() {
		for msg := range ch {
			utils.Info(fmt.Sprintf("Broadcaster: Message reçu de Redis sur le canal '%s'", msg.Channel))

			var event Message
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				utils.Error("Broadcaster: Erreur unmarshalling message Redis (" + msg.Channel + "): " + err.Error())
				continue
			}

			utils.Info(fmt.Sprintf("Broadcaster: Traitement de l'événement '%s' pour ServerID '%s', ChannelID '%s', UserID '%s'", event.Type, event.ServerID, event.ChannelID, event.UserID))

			clientsMutex.RLock()
			if len(clients) == 0 {
				utils.Warn("Broadcaster: Aucun client connecté pour la diffusion.")
			}

			for conn, client := range clients {
				utils.Info(fmt.Sprintf("Broadcaster: Vérification du client '%s' (UserID: %s) actuellement dans le serveur '%s', canal '%s'", client.Username, client.UserID, client.CurrentServerID, client.CurrentChannelID))

				switch event.Type {
				case "chat":
					if client.CurrentServerID == event.ServerID && client.CurrentChannelID == event.ChannelID {
						utils.Info(fmt.Sprintf("Broadcaster: CORRESPONDANCE CHAT ! Envoi du message à '%s' dans le serveur '%s', canal '%s'", client.Username, client.CurrentServerID, client.CurrentChannelID))
						if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
							utils.Error("Broadcaster: Erreur envoi message chat à client (" + client.Username + "): " + err.Error())
							conn.Close()
							clientsMutex.Lock()
							delete(clients, conn)
							clientsMutex.Unlock()
						}
					} else {
						utils.Warn(fmt.Sprintf("Broadcaster: PAS DE CORRESPONDANCE CHAT. Client S:%s/C:%s, Event S:%s/C:%s. Message non envoyé à %s.",
							client.CurrentServerID, client.CurrentChannelID, event.ServerID, event.ChannelID, client.Username))
					}
				case "presence":
					utils.Info(fmt.Sprintf("Broadcaster: Envoi de la mise à jour de présence pour user '%s' au client '%s'", event.UserID, client.Username))
					if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
						utils.Error("Broadcaster: Erreur envoi message présence à client (" + client.Username + "): " + err.Error())
						conn.Close()
						clientsMutex.Lock()
						delete(clients, conn)
						clientsMutex.Unlock()
					}
				case "server:presence:updates": // Exemple de message de présence spécifique au serveur
					if client.CurrentServerID == event.ServerID {
						utils.Info(fmt.Sprintf("Broadcaster: Envoi de la mise à jour de présence serveur à '%s' pour serveur '%s'", client.Username, event.ServerID))
						if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
							utils.Error("Broadcaster: Erreur envoi mise à jour présence serveur à client (" + client.Username + "): " + err.Error())
							conn.Close()
							clientsMutex.Lock()
							delete(clients, conn)
							clientsMutex.Unlock()
						}
					}
				default:
					utils.Warn("Broadcaster: Type d'événement inconnu reçu: " + event.Type)
				}
			}
			clientsMutex.RUnlock()
		}
		utils.Info("Broadcaster Redis: Le canal de messages a été fermé, goroutine arrêtée.")
	}()
}
