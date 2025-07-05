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

func GetChannelMessages(c *fiber.Ctx) error {
	// --- ÉTAPES 1 & 2: Validation et Permissions (inchangées) ---
	utils.Info("Début de GetChannelMessages")

	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		utils.Error("ID de serveur invalide", "id", serverIDStr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de serveur invalide."})
	}

	channelIDStr := c.Params("channelId")
	channelID, err := gocql.ParseUUID(channelIDStr)
	if err != nil {
		utils.Error("ID de salon invalide", "id", channelIDStr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de salon invalide."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var count int
	if err := db.Session.Query(`SELECT count(*) FROM server_members WHERE server_id = ? AND user_id = ? LIMIT 1`, serverID, userID).Scan(&count); err != nil {
		utils.Error("Erreur lors de la vérification des membres du serveur", "error", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur interne."})
	}

	if count == 0 {
		utils.Warn("Tentative d'accès non autorisé", "user_id", userID, "server_id", serverID)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Vous n'êtes pas membre de ce serveur."})
	}

	// --- ÉTAPE 3: Gestion de la pagination (revue et corrigée) ---
	limitStr := c.Query("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursorStr := c.Query("cursor")
	var startDate time.Time
	var cursorQueryPart string
	var cursorQueryArgs []interface{}

	if cursorStr != "" {
		cursor, err := gocql.ParseUUID(cursorStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Curseur de pagination invalide."})
		}
		// On extrait l'heure du TIMEUUID pour trouver le point de départ
		startDate = cursor.Time().UTC()
		cursorQueryPart = `AND sent_at < ?`
		cursorQueryArgs = []interface{}{cursor}
	} else {
		// Pas de curseur, on part d'aujourd'hui
		startDate = time.Now().UTC()
	}

	// --- ÉTAPE 4: Récupération des messages (boucle intelligente) ---
	messages := make([]MessageInfo, 0, limit)
	var lastMessageID gocql.UUID

	// On boucle en arrière dans le temps, jour par jour, à partir de startDate.
	// On met une limite raisonnable (ex: 5 ans) pour éviter une boucle infinie en cas d'erreur.
	for i := 0; i < 365*5; i++ {
		currentDate := startDate.AddDate(0, 0, -i)
		dayBucket := currentDate.Format("2006-01-02")

		query := fmt.Sprintf(`SELECT sent_at, sender_id, content FROM messages_by_channel WHERE channel_id = ? AND day_bucket = ? %s ORDER BY sent_at DESC LIMIT ?`, cursorQueryPart)

		// Le `LIMIT` est le nombre de messages qu'il nous reste à trouver
		remainingLimit := limit - len(messages)

		args := append([]interface{}{channelID, dayBucket}, cursorQueryArgs...)
		args = append(args, remainingLimit)

		iter := db.Session.Query(query, args...).Iter()

		var msgID, senderID gocql.UUID
		var content string

		tempMessages := make([]MessageInfo, 0)
		for iter.Scan(&msgID, &senderID, &content) {
			tempMessages = append(tempMessages, MessageInfo{
				MessageID: msgID,
				SenderID:  senderID,
				Content:   content,
				SentAt:    msgID.Time(),
			})
		}

		if err := iter.Close(); err != nil {
			utils.Error("Erreur lors de la lecture des messages", "error", err, "bucket", dayBucket)
			return c.Status(500).JSON(fiber.Map{"message": "Erreur lors de la récupération des messages."})
		}

		if len(tempMessages) > 0 {
			messages = append(messages, tempMessages...)
			// Le dernier message trouvé dans cette page devient le nouveau curseur potentiel.
			lastMessageID = tempMessages[len(tempMessages)-1].MessageID

			// Si on a assez de messages, on arrête.
			if len(messages) >= limit {
				break
			}

			// On met à jour le curseur pour la prochaine itération de la boucle (pour le jour J-1, J-2, etc.).
			// Cela assure une continuité parfaite entre les jours.
			cursorQueryPart = `AND sent_at < ?`
			cursorQueryArgs = []interface{}{lastMessageID}
		}
	}

	// --- ÉTAPE 5: Renvoyer la réponse ---
	var nextCursor string
	if len(messages) >= limit {
		// On renvoie l'ID du dernier message de la liste comme curseur pour la page suivante.
		nextCursor = messages[len(messages)-1].MessageID.String()
	}

	return c.JSON(fiber.Map{
		"messages":    messages,
		"next_cursor": nextCursor,
	})
}
