package handlers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// NOTE: Ces types struct devraient probablement vivre dans un package `models`
type serverMember struct {
	UserID   gocql.UUID `json:"user_id"`
	Username string     `json:"username"`
	Avatar   string     `json:"avatar"`
	Role     string     `json:"role"`
	JoinedAt time.Time  `json:"joined_at"`
}

type serverResponse struct {
	ServerID  gocql.UUID     `json:"server_id"`
	Name      string         `json:"name"`
	Avatar    string         `json:"avatar,omitempty"`
	OwnerID   gocql.UUID     `json:"owner_id"`
	CreatedAt time.Time      `json:"created_at"`
	Members   []serverMember `json:"members"`
}

// ----------------------
// 📌 Créer un serveur
// ----------------------
// ----------------------
// 📌 Créer un serveur
// ----------------------
func CreateServer(c *fiber.Ctx) error {
	// ✅ On lit les champs depuis un formulaire multipart
	serverName := c.FormValue("name")
	utils.Info(serverName)
	if strings.TrimSpace(serverName) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Le nom du serveur est requis (server)."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	gocqlUserID := gocql.UUID(*rawUserID)

	// La fonction helper gère l'upload depuis le champ "icon" du formulaire
	avatarURL, err := processAndUploadIcon(c, "icon", "server-icon-")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	serverID := gocql.TimeUUID()
	createdAt := time.Now()
	var username, userAvatar string
	if err := db.Session.Query(`SELECT username, avatar FROM users WHERE id = ?`, gocqlUserID).Scan(&username, &userAvatar); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Impossible de récupérer le profil."})
	}

	batch := db.Session.NewBatch(gocql.LoggedBatch)
	batch.Query(`INSERT INTO servers (server_id, name, owner_id, created_at, avatar) VALUES (?, ?, ?, ?, ?)`,
		serverID, serverName, gocqlUserID, createdAt, avatarURL)

	batch.Query(`INSERT INTO user_servers (user_id, server_id, role, joined_at, server_avatar) VALUES (?, ?, ?, ?, ?)`,
		gocqlUserID, serverID, "owner", createdAt, avatarURL)

	batch.Query(`INSERT INTO server_members (server_id, user_id, role, joined_at, username, avatar) VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, gocqlUserID, "owner", createdAt, username, userAvatar)

	if err := db.Session.ExecuteBatch(batch); err != nil {
		utils.Error("Server creation batch failed", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la création du serveur."})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":    "Serveur créé avec succès !",
		"server_id":  serverID.String(),
		"avatar_url": avatarURL,
	})
}

// ... (Les autres fonctions restent identiques mais sont incluses pour l'exhaustivité)

// ----------------------
// 📌 Joindre un serveur
// ----------------------
func JoinServer(c *fiber.Ctx) error {
	serverID, err := gocql.ParseUUID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de serveur invalide."})
	}
	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var count int
	if err := db.Session.Query(`SELECT count(*) FROM user_servers WHERE user_id = ? AND server_id = ?`, userID, serverID).Scan(&count); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur interne."})
	}
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "Vous êtes déjà membre de ce serveur."})
	}

	var serverAvatar string
	if err := db.Session.Query(`SELECT avatar FROM servers WHERE server_id = ?`, serverID).Scan(&serverAvatar); err != nil {
		if err == gocql.ErrNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Ce serveur n'existe pas."})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur de récupération du serveur."})
	}

	var username, userAvatar string
	if err := db.Session.Query(`SELECT username, avatar FROM users WHERE id = ?`, userID).Scan(&username, &userAvatar); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Impossible de récupérer le profil."})
	}

	joinedAt := time.Now()
	batch := db.Session.NewBatch(gocql.LoggedBatch)
	batch.Query(`INSERT INTO user_servers (user_id, server_id, role, joined_at, server_avatar) VALUES (?, ?, ?, ?, ?)`,
		userID, serverID, "member", joinedAt, serverAvatar)
	batch.Query(`INSERT INTO server_members (server_id, user_id, role, joined_at, username, avatar) VALUES (?, ?, ?, ?, ?, ?)`,
		serverID, userID, "member", joinedAt, username, userAvatar)

	if err := db.Session.ExecuteBatch(batch); err != nil {
		utils.Error("Server join batch failed", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur pour rejoindre le serveur."})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Serveur rejoint avec succès."})
}

// ----------------------
// 📌 Récupérer la liste des serveurs d'un utilisateur
// ----------------------
func GetUserServers(c *fiber.Ctx) error {
	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	type serverInfo struct {
		ServerID  gocql.UUID `json:"server_id"`
		Name      string     `json:"name"`
		Avatar    string     `json:"avatar,omitempty"`
		OwnerID   gocql.UUID `json:"owner_id"`
		Role      string     `json:"role"`
		JoinedAt  time.Time  `json:"joined_at"`
		CreatedAt time.Time  `json:"created_at"`
	}

	type userServerData struct {
		Role         string
		JoinedAt     time.Time
		ServerAvatar string
	}
	serverDetailsMap := make(map[gocql.UUID]userServerData)
	var serverIDs []gocql.UUID

	iter := db.Session.Query(`SELECT server_id, role, joined_at, server_avatar FROM user_servers WHERE user_id = ?`, userID).Iter()
	var sid gocql.UUID
	var role, serverAvatar string
	var joinedAt time.Time

	for iter.Scan(&sid, &role, &joinedAt, &serverAvatar) {
		serverIDs = append(serverIDs, sid)
		serverDetailsMap[sid] = userServerData{Role: role, JoinedAt: joinedAt, ServerAvatar: serverAvatar}
	}
	if err := iter.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "Erreur #1."})
	}

	if len(serverIDs) == 0 {
		return c.JSON([]interface{}{})
	}

	results := make([]serverInfo, 0, len(serverIDs))
	iter = db.Session.Query(`SELECT server_id, name, owner_id, created_at FROM servers WHERE server_id IN ?`, serverIDs).Iter()
	var name string
	var ownerID gocql.UUID
	var createdAt time.Time

	for iter.Scan(&sid, &name, &ownerID, &createdAt) {
		details := serverDetailsMap[sid]
		results = append(results, serverInfo{
			ServerID:  sid,
			Name:      name,
			Avatar:    details.ServerAvatar,
			OwnerID:   ownerID,
			Role:      details.Role,
			JoinedAt:  details.JoinedAt,
			CreatedAt: createdAt,
		})
	}
	if err := iter.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "Erreur #2."})
	}

	return c.JSON(results)
}

// ----------------------
// 📌 Get infos détaillées d'un serveur
// ----------------------
func GetServer(c *fiber.Ctx) error {
	serverID, err := gocql.ParseUUID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID invalide."})
	}

	var response serverResponse
	if err := db.Session.Query(
		`SELECT server_id, name, owner_id, created_at, avatar FROM servers WHERE server_id = ?`,
		serverID,
	).Scan(&response.ServerID, &response.Name, &response.OwnerID, &response.CreatedAt, &response.Avatar); err != nil {
		if err == gocql.ErrNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Serveur introuvable."})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur interne."})
	}

	response.Members = make([]serverMember, 0)
	iter := db.Session.Query(`SELECT user_id, username, avatar, role, joined_at FROM server_members WHERE server_id = ?`, serverID).Iter()
	var member serverMember
	for iter.Scan(&member.UserID, &member.Username, &member.Avatar, &member.Role, &member.JoinedAt) {
		response.Members = append(response.Members, member)
	}
	if err := iter.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "Erreur membres."})
	}

	return c.JSON(response)
}

// ----------------------
// 📌 Modifier un serveur (nom et/ou avatar)
// ----------------------
func UpdateServer(c *fiber.Ctx) error {
	serverID, err := gocql.ParseUUID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID invalide."})
	}

	rawUserID := c.Locals("user_id").(*uuid.UUID)
	userID := gocql.UUID(*rawUserID)

	var ownerID gocql.UUID
	var oldAvatarURL string
	if err := db.Session.Query(`SELECT owner_id, avatar FROM servers WHERE server_id = ?`, serverID).Scan(&ownerID, &oldAvatarURL); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Serveur introuvable."})
	}
	if ownerID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Action non autorisée."})
	}

	newName := c.FormValue("name")
	newAvatarURL, err := processAndUploadIcon(c, "icon", "server-icon-")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
	}

	if newName == "" && newAvatarURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Aucune modification fournie."})
	}

	batch := db.Session.NewBatch(gocql.LoggedBatch)
	if newName != "" {
		batch.Query(`UPDATE servers SET name = ? WHERE server_id = ?`, newName, serverID)
	}
	if newAvatarURL != "" {
		batch.Query(`UPDATE servers SET avatar = ? WHERE server_id = ?`, newAvatarURL, serverID)
	}

	if newAvatarURL != "" {
		var memberIDs []gocql.UUID
		iter := db.Session.Query(`SELECT user_id FROM server_members WHERE server_id = ?`, serverID).Iter()
		var memberID gocql.UUID
		for iter.Scan(&memberID) {
			memberIDs = append(memberIDs, memberID)
		}
		_ = iter.Close()

		for _, id := range memberIDs {
			batch.Query(`UPDATE user_servers SET server_avatar = ? WHERE user_id = ? AND server_id = ?`, newAvatarURL, id, serverID)
		}
	}

	if err := db.Session.ExecuteBatch(batch); err != nil {
		utils.Error("Server update batch failed", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la modification."})
	}

	if newAvatarURL != "" && oldAvatarURL != "" {
		go utils.DeleteObject(oldAvatarURL)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Serveur mis à jour !"})
}

// ----------------------
// 📌 Supprimer un serveur
// ----------------------
func DeleteServer(c *fiber.Ctx) error {
	serverID, err := gocql.ParseUUID(c.Params("id"))
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

	var memberIDs []gocql.UUID
	iter := db.Session.Query(`SELECT user_id FROM server_members WHERE server_id = ?`, serverID).Iter()
	var memberID gocql.UUID
	for iter.Scan(&memberID) {
		memberIDs = append(memberIDs, memberID)
	}
	_ = iter.Close()

	// ✅ Le batch est bien initialisé ici avant d'être utilisé
	batch := db.Session.NewBatch(gocql.LoggedBatch)

	batch.Query(`DELETE FROM servers WHERE server_id = ?`, serverID)
	batch.Query(`DELETE FROM server_members WHERE server_id = ?`, serverID)
	batch.Query(`DELETE FROM categories_by_server WHERE server_id = ?`, serverID)
	batch.Query(`DELETE FROM channels_by_server WHERE server_id = ?`, serverID)

	for _, id := range memberIDs {
		batch.Query(`DELETE FROM user_servers WHERE user_id = ? AND server_id = ?`, id, serverID)
	}

	if err := db.Session.ExecuteBatch(batch); err != nil {
		utils.Error("Server deletion batch failed", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors de la suppression."})
	}

	if avatarURL != "" {
		go utils.DeleteObject(avatarURL)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Serveur supprimé avec succès."})
}

// ----------------------
// 헬 Helper pour l'upload d'image
// ----------------------
func processAndUploadIcon(c *fiber.Ctx, formKey, namePrefix string) (string, error) {
	file, err := c.FormFile(formKey)
	if err != nil {
		return "", nil
	}

	src, _ := file.Open()
	defer src.Close()

	cfg := config.GetConfig()
	contentType := file.Header.Get("Content-Type")
	var converted *bytes.Buffer

	if strings.HasSuffix(strings.ToLower(file.Filename), ".gif") {
		converted, err = utils.ConvertToRoundedGIF(src)
	} else {
		converted, err = utils.ConvertToRoundedWebP(src, contentType)
	}
	if err != nil {
		utils.Error("Icon conversion failed", "err", err)
		return "", fmt.Errorf("erreur de traitement de l'image")
	}

	randStr, _ := utils.RandomString64()
	fileName := namePrefix + randStr + ".webp"
	_, err = utils.MinioClient.PutObject(context.Background(), cfg.MinioBucket, fileName, converted, int64(converted.Len()), minio.PutObjectOptions{
		ContentType: "image/webp",
	})
	if err != nil {
		utils.Error("MinIO icon upload failed", "err", err, "fileName", fileName)
		return "", fmt.Errorf("erreur d'upload de l'icône")
	}

	return fmt.Sprintf("%s/%s", cfg.MinioURL, fileName), nil
}
