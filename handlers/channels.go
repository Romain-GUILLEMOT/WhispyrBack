package handlers

import (
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
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

// --- Handlers ---

// GetServerChannelsAndCategories récupère la liste structurée des catégories et salons.
func GetServerChannelsAndCategories(c *fiber.Ctx) error {
	// --- LOG DE DÉBUT ---
	utils.Info("Début de GetServerChannelsAndCategories")

	serverIDStr := c.Params("serverId")
	serverID, err := gocql.ParseUUID(serverIDStr)
	if err != nil {
		utils.Error("ID de serveur invalide fourni", "id", serverIDStr)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de serveur invalide."})
	}
	utils.Info(fmt.Sprintf("Récupération des catégories pour le serveur: %s", serverID))

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
	// On vérifie l'erreur APRÈS la boucle, c'est la bonne pratique avec gocql
	if err := catIter.Close(); err != nil {
		utils.Error("Erreur lors de la fermeture de l'itérateur de catégories", "error", err)
		return c.Status(500).JSON(fiber.Map{"message": "Erreur lors de la lecture des catégories."})
	}
	utils.Info(fmt.Sprintf("Trouvé %d catégories pour le serveur %s", len(orderedCategories), serverID))

	// --- ÉTAPE 2: Récupérer les salons et les assigner ---
	utils.Info("Récupération des salons pour le serveur...")
	chanIter := db.Session.Query(`SELECT category_id, channel_id, name, type, is_private, position FROM channels_by_server WHERE server_id = ?`, serverID).Iter()

	// *** CORRECTION ICI : On déclare les variables manquantes pour le scan ***
	var chanID gocql.UUID
	var chanName, chanType string
	var isPrivate bool
	var chanPos int

	// *** CORRECTION ICI : On scanne dans TOUTES les variables ***
	for chanIter.Scan(&catID, &chanID, &chanName, &chanType, &isPrivate, &chanPos) {
		if category, ok := categoriesMap[catID]; ok {
			channel := ChannelInfo{
				ChannelID:  chanID,
				Name:       chanName,
				Type:       chanType,
				IsPrivate:  isPrivate,
				Position:   chanPos,
				CategoryID: catID,
			}
			category.Channels = append(category.Channels, channel)
		} else {
			// Log si un salon a une catégorie qui n'existe pas (incohérence de données)
			utils.Warn(fmt.Sprintf("Salon %s a une categoryID %s non trouvée", chanID, catID))
		}
	}
	if err := chanIter.Close(); err != nil {
		// C'est ici que l'erreur se produisait. Maintenant, elle devrait être plus explicite si elle persiste.
		utils.Error("Erreur lors de la fermeture de l'itérateur de salons", "error", err)
		return c.Status(500).JSON(fiber.Map{"message": "Erreur lors de la lecture des salons."})
	}
	utils.Info("Lecture des salons terminée avec succès.")

	return c.JSON(orderedCategories)
}

// CreateChannel crée un nouveau salon dans une catégorie.
func CreateChannel(c *fiber.Ctx) error {
	// TODO: Ajouter une vérification des permissions (seul un admin peut créer un salon)

	var reqBody struct {
		ServerID   string `json:"server_id"`
		CategoryID string `json:"category_id"`
		Name       string `json:"name"`
		Type       string `json:"type"` // "text" ou "voice"
	}

	if err := c.BodyParser(&reqBody); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Données invalides."})
	}

	if err := dbTools.CreateChannelInDB(reqBody.ServerID, reqBody.CategoryID, reqBody.Name, reqBody.Type); err != nil {
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

	if err := dbTools.UpdateChannelInDB(channelID, reqBody.Name); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la mise à jour du salon."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Salon mis à jour."})
}

// DeleteChannel supprime un salon.
func DeleteChannel(c *fiber.Ctx) error {
	// TODO: Ajouter une vérification des permissions

	channelID := c.Params("id")
	if err := dbTools.DeleteChannelFromDB(channelID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la suppression du salon."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Salon supprimé."})
}
