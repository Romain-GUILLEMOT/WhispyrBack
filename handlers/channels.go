package handlers

import (
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"sort"
	"strconv"
	"strings"
	"time"
)

// --- Structures de réponse ---

type ChannelInfo struct {
	ChannelID  gocql.UUID `json:"channel_id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	IsPrivate  bool       `json:"is_private"`
	Position   int        `json:"position"`
	CategoryID gocql.UUID `json:"category_id"`
}

type CategoryWithChannels struct {
	CategoryID gocql.UUID    `json:"category_id"`
	Name       string        `json:"name"`
	Position   int           `json:"position"`
	Channels   []ChannelInfo `json:"channels"`
}
type MessageInfo struct {
	MessageID gocql.UUID `json:"message_id"`
	SenderID  gocql.UUID `json:"sender_id"`
	// On pourrait ajouter username/avatar ici si on voulait dénormaliser davantage
	Content string    `json:"content"`
	SentAt  time.Time `json:"sent_at"`
}

type MessageResponse struct {
	ID             gocql.UUID `json:"id"`
	Content        string     `json:"content"`
	Timestamp      time.Time  `json:"timestamp"`
	SenderID       gocql.UUID `json:"sender_id"`
	SenderUsername string     `json:"username"`
	SenderAvatar   string     `json:"avatar"`
}

// --- Handlers ---

// GetServerChannelsAndCategories récupère la liste structurée des catégories et salons.
func GetServerChannelsAndCategories(c *fiber.Ctx) error {
	utils.Info("Début de GetServerChannelsAndCategories")

	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		utils.Error("ID de serveur invalide fourni", "id", serverIDStr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de serveur invalide."})
	}
	utils.Info(fmt.Sprintf("Récupération des données pour le serveur: %s", serverID))

	// --- ÉTAPE 1: Récupérer les catégories ---
	categoriesMap := make(map[gocql.UUID]*CategoryWithChannels)
	var orderedCategories []*CategoryWithChannels

	catIter := db.Session.Query(`SELECT category_id, name, position FROM categories_by_server WHERE server_id = ?`, serverID).Iter()
	var catID gocql.UUID
	var catName string
	var catPos int

	for catIter.Scan(&catID, &catName, &catPos) {
		category := &CategoryWithChannels{
			CategoryID: catID,
			Name:       catName,
			Position:   catPos,
			Channels:   make([]ChannelInfo, 0),
		}
		categoriesMap[catID] = category
		orderedCategories = append(orderedCategories, category)
	}
	if err := catIter.Close(); err != nil {
		utils.Error("Erreur lors de la fermeture de l'itérateur de catégories", "error", err)
		return c.Status(500).JSON(fiber.Map{"message": "Erreur lors de la lecture des catégories."})
	}
	utils.Info(fmt.Sprintf("Trouvé %d catégories pour le serveur %s", len(orderedCategories), serverID))

	// --- ÉTAPE 2: Récupérer les salons et les assigner ---
	var uncategorizedChannels []ChannelInfo // Liste pour les salons sans catégorie trouvée

	chanIter := db.Session.Query(`SELECT category_id, channel_id, name, type, is_private, position FROM channels_by_server WHERE server_id = ?`, serverID).Iter()
	var chanID gocql.UUID
	var chanName, chanType string
	var isPrivate bool
	var chanPos int

	for chanIter.Scan(&catID, &chanID, &chanName, &chanType, &isPrivate, &chanPos) {
		channel := ChannelInfo{
			ChannelID:  chanID,
			Name:       chanName,
			Type:       chanType,
			IsPrivate:  isPrivate,
			Position:   chanPos,
			CategoryID: catID,
		}

		if category, ok := categoriesMap[catID]; ok {
			// Le salon a une catégorie valide, on l'ajoute.
			category.Channels = append(category.Channels, channel)
		} else {
			// Le salon est orphelin, on l'ajoute à la liste dédiée.
			utils.Warn(fmt.Sprintf("Salon %s a une categoryID %s non trouvée. Ajouté aux non-catégorisés.", chanID, catID))
			uncategorizedChannels = append(uncategorizedChannels, channel)
		}
	}
	if err := chanIter.Close(); err != nil {
		utils.Error("Erreur lors de la fermeture de l'itérateur de salons", "error", err)
		return c.Status(500).JSON(fiber.Map{"message": "Erreur lors de la lecture des salons."})
	}
	utils.Info(fmt.Sprintf("Lecture de %d salons terminée. %d sont non-catégorisés.", len(orderedCategories)+len(uncategorizedChannels), len(uncategorizedChannels)))

	// --- ÉTAPE 3: Trier les canaux au sein de chaque catégorie ---
	for _, category := range orderedCategories {
		sort.Slice(category.Channels, func(i, j int) bool {
			return category.Channels[i].Position < category.Channels[j].Position
		})
	}

	// --- ÉTAPE 4: Construire et renvoyer la réponse finale ---
	return c.JSON(fiber.Map{
		"categories":             orderedCategories,
		"uncategorized_channels": uncategorizedChannels,
	})
}

// CreateChannel crée un nouveau salon dans une catégorie.
func CreateChannel(c *fiber.Ctx) error {
	// TODO: Ajouter une vérification des permissions (seul un admin peut créer un salon)

	var reqBody struct {
		CategoryID string `json:"category_id"`
		Name       string `json:"name"`
		Type       string `json:"type"` // "text" ou "voice"
	}

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Données invalides."})
	}
	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID invalide."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var ownerID gocql.UUID
	var avatarURL string
	if err := db.Session.Query(`SELECT owner_id, avatar FROM servers WHERE server_id = ?`, serverID).Scan(&ownerID, &avatarURL); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Serveur introuvable."})
	}
	if ownerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Action non autorisée."})
	}

	if err := dbTools.CreateChannelInDB(serverIDStr, reqBody.CategoryID, reqBody.Name, reqBody.Type); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la création du salon."})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Salon créé avec succès."})
}

// UpdateChannel modifie un salon existant.
func UpdateChannel(c *fiber.Ctx) error {
	// TODO: Ajouter une vérification des permissions

	channelID := c.Params("id")
	var reqBody struct {
		Name string `json:"name"`
		// On pourrait ajouter la position, etc.
	}
	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Données invalides."})
	}
	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID invalide."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var ownerID gocql.UUID
	var avatarURL string
	if err := db.Session.Query(`SELECT owner_id, avatar FROM servers WHERE server_id = ?`, serverID).Scan(&ownerID, &avatarURL); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Serveur introuvable."})
	}
	if ownerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Action non autorisée."})
	}

	if err := dbTools.UpdateChannelInDB(channelID, reqBody.Name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la mise à jour du salon."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Salon mis à jour."})
}

// DeleteChannel supprime un salon.
func DeleteChannel(c *fiber.Ctx) error {
	// TODO: Ajouter une vérification des permissions

	channelID := c.Params("id")
	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID invalide."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var ownerID gocql.UUID
	var avatarURL string
	if err := db.Session.Query(`SELECT owner_id, avatar FROM servers WHERE server_id = ?`, serverID).Scan(&ownerID, &avatarURL); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Serveur introuvable."})
	}
	if ownerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Action non autorisée."})
	}

	if err := dbTools.DeleteChannelFromDB(channelID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la suppression du salon."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Salon supprimé."})
}

// GetChannelMessages gère la récupération paginée des messages pour un salon.
func GetChannelMessages(c *fiber.Ctx) error {
	utils.Info("Début de GetChannelMessages")

	// --- Validation et Permissions ---
	serverID, err := gocql.ParseUUID(c.Params("serverId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de serveur invalide."})
	}
	channelID, err := gocql.ParseUUID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de salon invalide."})
	}

	userIDClaim := c.Locals("user_id")
	if userIDClaim == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Non authentifié."})
	}

	var userID gocql.UUID
	var ok bool
	userID, ok = userIDClaim.(gocql.UUID)
	if !ok {
		if userIDPtr, ok := userIDClaim.(*gocql.UUID); ok && userIDPtr != nil {
			userID = *userIDPtr
		} else if googleUUIDPtr, ok := userIDClaim.(*uuid.UUID); ok && googleUUIDPtr != nil {
			userID = gocql.UUID(*googleUUIDPtr)
		} else if userIDStr, ok := userIDClaim.(string); ok {
			userID, err = gocql.ParseUUID(userIDStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Format d'ID utilisateur invalide dans le token."})
			}
		} else {
			utils.Error("Type de user_id non géré", "type", fmt.Sprintf("%T", userIDClaim))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Type d'ID utilisateur invalide en contexte."})
		}
	}

	var foundServer gocql.UUID
	if err := db.Session.Query(`SELECT server_id FROM server_members WHERE server_id = ? AND user_id = ? LIMIT 1`, serverID, userID).Scan(&foundServer); err != nil {
		if err == gocql.ErrNotFound {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Vous n'êtes pas membre de ce serveur."})
		}
		utils.Error("Erreur lors de la vérification des membres du serveur", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur interne."})
	}

	// --- Pagination et Limite ---
	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}
	cursorStr := c.Query("cursor")
	// On fetch beaucoup plus de messages pour pouvoir filtrer et avoir assez de messages pour une page.
	fetchLimit := limit * 5

	// --- Logique de Day Bucket ---
	const daysToScan = 30 // Nombre de jours à scanner en arrière
	dayBuckets := make([]string, 0, daysToScan)
	startDate := time.Now().UTC()
	for i := 0; i < daysToScan; i++ {
		dayBuckets = append(dayBuckets, startDate.AddDate(0, 0, -i).Format("2006-01-02"))
	}

	// --- Construction de la requête ---
	placeholders := strings.Repeat("?,", len(dayBuckets)-1) + "?"
	query := fmt.Sprintf(`SELECT sent_at, sender_id, content, sender_username, sender_avatar 
                          FROM messages_by_channel 
                          WHERE channel_id = ? AND day_bucket IN (%s) 
                          LIMIT ?`, placeholders)

	args := make([]interface{}, 1+len(dayBuckets)+1)
	args[0] = channelID
	for i, bucket := range dayBuckets {
		args[i+1] = bucket
	}
	args[len(args)-1] = fetchLimit

	iter := db.Session.Query(query, args...).Iter()

	// --- Scan des résultats ---
	allMessages := make([]MessageResponse, 0, fetchLimit)
	var msgID, senderID gocql.UUID
	var content string
	var senderUsername, senderAvatar *string

	for iter.Scan(&msgID, &senderID, &content, &senderUsername, &senderAvatar) {
		var finalUsername, finalAvatar string
		if senderUsername != nil {
			finalUsername = *senderUsername
		}
		if senderAvatar != nil {
			finalAvatar = *senderAvatar
		}
		allMessages = append(allMessages, MessageResponse{
			ID:             msgID,
			Content:        content,
			Timestamp:      msgID.Time(),
			SenderID:       senderID,
			SenderUsername: finalUsername,
			SenderAvatar:   finalAvatar,
		})
	}

	if err := iter.Close(); err != nil {
		utils.Error("Erreur lors de la lecture des messages", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la récupération des messages."})
	}

	// --- Tri et Filtrage en mémoire ---
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp.After(allMessages[j].Timestamp)
	})

	var messagesToConsider []MessageResponse
	if cursorStr != "" {
		cursorUUID, err := gocql.ParseUUID(cursorStr)
		if err == nil {
			cursorTime := cursorUUID.Time()
			for _, msg := range allMessages {
				if msg.Timestamp.Before(cursorTime) {
					messagesToConsider = append(messagesToConsider, msg)
				}
			}
		} else {
			messagesToConsider = allMessages
		}
	} else {
		messagesToConsider = allMessages
	}

	// --- Détermination de la page finale et du prochain curseur ---
	var finalMessages []MessageResponse
	var nextCursor string
	if len(messagesToConsider) > limit {
		finalMessages = messagesToConsider[:limit]
		nextCursor = finalMessages[len(finalMessages)-1].ID.String()
	} else {
		finalMessages = messagesToConsider
	}

	return c.JSON(fiber.Map{
		"data":        finalMessages,
		"next_cursor": nextCursor,
	})
}
