package auth

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"strings"
)

type LoginUserInput struct {
	EmailOrUsername string `json:"email" validate:"required"`
	Password        string `json:"password" validate:"required"`
}

// LoginUserInput doit être défini quelque part dans ton package, par exemple :
// type LoginUserInput struct {
//	 EmailOrUsername string `json:"email_or_username" validate:"required"`
//	 Password        string `json:"password" validate:"required"`
// }

func LoginUser(c *fiber.Ctx) error {
	var input LoginUserInput
	if err := c.BodyParser(&input); err != nil {
		utils.Error(err.Error())
		return fiber.NewError(fiber.StatusBadRequest, "Requête invalide. (Code: LOGIN-001)")
	}
	if err := validate.Struct(input); err != nil {
		utils.Error(err.Error())
		return fiber.NewError(fiber.StatusBadRequest, "Champs invalides. (Code: LOGIN-002)")
	}

	var userID gocql.UUID
	loginIdentifier := input.EmailOrUsername

	// --- ÉTAPE 1: Lookup rapide via la table d'index appropriée ---
	if strings.Contains(loginIdentifier, "@") {
		// L'identifiant est un email, on interroge users_by_email
		err := db.Session.Query(
			`SELECT id FROM users_by_email WHERE email = ? LIMIT 1`,
			strings.ToLower(loginIdentifier),
		).Scan(&userID)
		if err != nil {
			if err == gocql.ErrNotFound {
				return fiber.NewError(fiber.StatusUnauthorized, "Identifiants incorrects. (Code: LOGIN-003)")
			}
			utils.Error("ScyllaDB lookup by email failed", "err", err)
			return fiber.NewError(fiber.StatusInternalServerError, "Erreur interne. (Code: LOGIN-007)")
		}
	} else {
		// L'identifiant est un username, on interroge users_by_username
		err := db.Session.Query(
			`SELECT id FROM users_by_username WHERE username = ? LIMIT 1`,
			loginIdentifier,
		).Scan(&userID)
		if err != nil {
			if err == gocql.ErrNotFound {
				return fiber.NewError(fiber.StatusUnauthorized, "Identifiants incorrects. (Code: LOGIN-003)")
			}
			utils.Error("ScyllaDB lookup by username failed", "err", err)
			return fiber.NewError(fiber.StatusInternalServerError, "Erreur interne. (Code: LOGIN-008)")
		}
	}

	// --- ÉTAPE 2: Fetch des données complètes via l'ID (requête sur clé primaire) ---
	var user models.User
	if err := db.Session.Query(
		`SELECT id, email, username, avatar, password FROM users WHERE id = ? LIMIT 1`,
		userID,
	).Scan(&user.ID, &user.Email, &user.Username, &user.Avatar, &user.Password); err != nil {
		// Ce cas est très peu probable si l'étape 1 a réussi, mais c'est une sécurité
		utils.Error("ScyllaDB fetch by ID failed after lookup", "err", err, "userID", userID)
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur de cohérence des données. (Code: LOGIN-009)")
	}

	// --- ÉTAPE 3: Vérification du mot de passe ---
	if ok := utils.CheckPasswordHash(input.Password, user.Password); !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Identifiants incorrects. (Code: LOGIN-004)")
	}

	// --- ÉTAPE 4: Génération du DeviceID et des Tokens ---
	ua := c.Get("User-Agent")
	accept := c.Get("Accept")
	lang := c.Get("Accept-Language")
	encoding := c.Get("Accept-Encoding")
	ip := c.IP()
	deviceID := utils.GenerateDeviceID(ua, accept, lang, encoding, ip)

	accessToken, err := utils.GenerateAccessToken(userID.String(), deviceID)
	if err != nil {
		utils.Error("Erreur génération accessToken", "err", err)
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur interne. (Code: LOGIN-005)")
	}
	refreshToken, err := utils.GenerateRefreshToken(userID.String(), deviceID)
	if err != nil {
		utils.Error("Erreur génération refreshToken", "err", err)
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur interne. (Code: LOGIN-006)")
	}

	// --- ÉTAPE 5: Réponse finale ---
	cfg := config.GetConfig()
	response := fiber.Map{
		"message":       "Connexion réussie ✅",
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": fiber.Map{
			"id":       user.ID.String(),
			"email":    user.Email,
			"username": user.Username,
			"avatar":   user.Avatar,
		},
	}

	if cfg.Debug {
		response["device_id"] = deviceID
	}

	return c.Status(200).JSON(response)
}
