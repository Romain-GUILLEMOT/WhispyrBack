package auth

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/htmlemail"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"math/rand"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var validate = validator.New()

type RegisterUserInput struct {
	Username string `json:"username" validate:"required,min=3,max=32"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Code     string `json:"code" validate:"required,len=4"`
}

func RegisterAskCode(c *fiber.Ctx) error {
	type request struct {
		Email string `validate:"required,email"`
	}

	email := c.Query("email")
	body := request{Email: email}
	utils.Info(email)
	if email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "L’email est requis. (Code: WHIAUTH-001)",
		})
	}
	if err := validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "L’adresse email fournie est invalide ou mal formée. (Code: WHIAUTH-002)",
		})
	}
	email = strings.ToLower(body.Email)
	err := utils.GetEmailDomain(email)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error() + " (Code: WHIAUTH-003)",
		})
	}
	cfg := config.GetConfig()

	var existingID gocql.UUID
	err = db.Session.Query(`SELECT id FROM users_by_email WHERE email = ? LIMIT 1`, email).Scan(&existingID)
	if err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Un compte avec cet email existe déjà. (Code: WHIAUTH-004)",
		})
	}

	// Cas 2 : L'erreur est AUTRE CHOSE que "non trouvé". C'est une vraie erreur système.
	if err != gocql.ErrNotFound {
		utils.Error("ScyllaDB query failed when checking for email", "err", err, "email", email)
		// On ne laisse SURTOUT PAS passer une inscription si la DB est en erreur.
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Erreur interne du service. (Code: WHIAUTH-030)",
		})
	}
	utils.Info(err.Error())

	code := fmt.Sprintf("%04d", rand.Intn(10000))
	key := "email_verif:" + email
	ttl, err := utils.RedisTTL(key)
	if cfg.Debug {
		utils.Info("TTL", "ttl", ttl)
		if cfg.Debug {
			err := utils.RedisDel(key)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"message": err.Error(),
				})
			}
		}
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Erreur interne. (Code: WHIAUTH-027)",
		})
	}
	if ttl > 0 {
		return c.Status(400).JSON(fiber.Map{
			"message": "Un code a déjà été envoyé à cet email. (Code: WHIAUTH-029)",
			"data":    ttl,
		})
	}

	err = utils.RedisSet(key, code, 5*time.Minute)
	if err != nil {
		utils.Error("Redis set code failed", "err", err)
		utils.SendErrorMail("142", "register.go", "Redis set code failed", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-005)"})
	}
	htmlBody, err := htmlemail.Verifcode(code)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-006)"})
	}
	err = utils.SendMail(email, "Whispyr - Code de vérification", htmlBody)
	if err != nil {
		utils.Error("Cannot send email", "err", err)
		utils.SendErrorMail("143", "register.go", "Cannot send email", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-006)"})
	}
	utils.Info(code)
	return c.Status(200).JSON(fiber.Map{
		"message": "Va checker tes mails 📧, ton code t’y attend !",
	})
}

func RegisterVerifyCode(c *fiber.Ctx) error {
	type request struct {
		Email string `json:"email" validate:"required,email"`
		Code  string `json:"code" validate:"required,len=4"`
	}

	var body request
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "La requête est invalide. Merci de vérifier les données envoyées. (Code: WHIAUTH-024)",
		})
	}
	if err := validate.Struct(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "La requête est invalide. Merci de vérifier les données envoyées. (Code: WHIAUTH-023)",
		})
	}
	email := strings.ToLower(body.Email)

	var existing string
	err := db.Session.Query(`SELECT email FROM users WHERE email = ? LIMIT 1`, email).Scan(&existing)
	if err == nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Un compte avec cet email existe déjà. (Code: WHIAUTH-022)",
		})
	}
	code := body.Code
	key := "email_verif:" + email
	exists, err := utils.RedisExists(key)
	if err != nil {
		utils.Error("Redis exist code failed", "err", err)
		utils.SendErrorMail("144", "register.go", "Redis exist code failed", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-021)"})
	}
	if !exists {
		return c.Status(400).JSON(fiber.Map{"message": "Code expiré.(Code: WHIAUTH-007)"})
	}
	storedCode, err := utils.RedisGet(key)
	if err != nil {
		utils.Error("Redis get code failed", "err", err)
		utils.SendErrorMail("145", "register.go", "Redis get code failed", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-020)"})
	}
	if storedCode != code {
		return c.Status(400).JSON(fiber.Map{"message": "Code invalide. (Code: WHIAUTH-008)"})
	}
	err = utils.RedisDel(key)
	if err != nil {
		utils.Error("Redis del code failed", "err", err)
		utils.SendErrorMail("146", "register.go", "Redis del code failed", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-009)"})
	}
	//create new redis code
	code = fmt.Sprintf("%04d", rand.Intn(10000))
	key = "acc_reg:" + email

	err = utils.RedisSet(key, code, 30*time.Minute)
	if err != nil {
		utils.Error("Redis set code failed", "err", err)
		utils.SendErrorMail("142", "register.go", "Redis set code failed", err.Error())
		return c.Status(500).JSON(fiber.Map{"message": "Erreur interne. (Code: WHIAUTH-010)"})
	}
	return c.Status(200).JSON(fiber.Map{
		"message": "🔑 Code vérifié",
		"data":    code,
	})
}

func RegisterUser(c *fiber.Ctx) error {
	username := c.FormValue("username")
	email := c.FormValue("email")
	password := c.FormValue("password")
	code := c.FormValue("code")
	input := RegisterUserInput{
		Username: username,
		Email:    email,
		Password: password,
		Code:     code,
	}
	cfg := config.GetConfig()

	if err := validate.Struct(input); err != nil {
		if cfg.Debug {
			utils.Error(err.Error())
		}
		return fiber.NewError(fiber.StatusBadRequest, "Champs invalides. (Code: WHIREG-001)")
	}

	key := "acc_reg:" + email
	if !cfg.Debug {
		storedCode, err := utils.RedisGet(key)
		if err != nil || storedCode != code {
			return c.Status(400).JSON(fiber.Map{
				"message": "Code expiré ou invalide. (Code: REG-002)",
			})
		}
	}

	// Vérifie si un compte existe déjà
	var existing string
	if err := db.Session.Query(`SELECT email FROM users WHERE email = ? LIMIT 1`, email).Scan(&existing); err == nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Un compte avec cet email existe déjà. (Code: REG-003)",
		})
	}

	// Upload de l’avatar unique
	var avatarURL string
	avatarURL = generateDefaultAvatarURL(username)
	file, err := c.FormFile("avatar")

	if err == nil {
		src, _ := file.Open()
		defer src.Close()
		ext := strings.ToLower(filepath.Ext(file.Filename)) // ex: ".gif"
		var converted *bytes.Buffer = nil
		if ext == ".gif" {
			converted, err = utils.ConvertToRoundedGIF(src)
			if err != nil {
				if cfg.Debug {
					utils.Error(err.Error())
				}
				return fiber.NewError(fiber.StatusInternalServerError, "Erreur conversion AVIF. (Code: REG-004)")
			}
		} else {
			converted, err = utils.ConvertToRoundedWebP(src, file.Header.Get("Content-Type"))
			if err != nil {
				if cfg.Debug {
					utils.Error(err.Error())
				}

				return fiber.NewError(fiber.StatusInternalServerError, "Erreur conversion WebP. (Code: REG-004)")
			}
		}
		randStr, _ := utils.RandomString64()
		name := randStr + ".webp"
		_, err = utils.MinioClient.PutObject(context.Background(), cfg.MinioBucket, name, converted, int64(converted.Len()), minio.PutObjectOptions{
			ContentType: "image/webp",
		})
		if err != nil {
			utils.Error("Échec MinIO PutObject", "err", err, "fichier", name)
			return fiber.NewError(fiber.StatusInternalServerError, "Erreur upload MinIO. (Code: REG-005)")
		}
		avatarURL = fmt.Sprintf(cfg.MinioURL+"/%s", name)
	}

	// Hash du mot de passe
	hashedPass, err := utils.HashPassword(input.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur de sécurité. (Code: REG-006)")
	}
	scyllaUUID := gocql.UUID(uuid.New())

	user := models.User{
		ID:        scyllaUUID,
		Username:  input.Username,
		Email:     email,
		Password:  hashedPass,
		Avatar:    avatarURL,
		CreatedAt: time.Now(),
	}

	// Insert dans ScyllaDB
	// --- ÉTAPE 1: Tenter de réserver l'email ---
	// L'ordre des variables DOIT correspondre à l'ordre des colonnes de la table.
	// Ordre pour `users_by_email`: email, avatar, id, username
	var conflictEmail, conflictAvatar, conflictUsername string
	var conflictID gocql.UUID

	applied, err := db.Session.Query(
		`INSERT INTO users_by_email (email, id, username, avatar) VALUES (?, ?, ?, ?) IF NOT EXISTS`,
		user.Email, user.ID, user.Username, user.Avatar,
	).WithContext(context.Background()).ScanCAS(&conflictEmail, &conflictAvatar, &conflictID, &conflictUsername)

	if err != nil {
		utils.Error("ScyllaDB LWT failed for users_by_email", "err", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur base de données. (Code: REG-010)"})
	}
	if !applied {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "Cet email est déjà utilisé. (Code: REG-003)"})
	}

	// --- ÉTAPE 2: Tenter de réserver le username ---
	// La table `users_by_username` a 3 colonnes. Assumons l'ordre : username, avatar, id
	var conflictUsername2, conflictAvatar2 string
	var conflictID2 gocql.UUID

	applied, err = db.Session.Query(
		`INSERT INTO users_by_username (username, id, avatar) VALUES (?, ?, ?) IF NOT EXISTS`,
		user.Username, user.ID, user.Avatar,
	).WithContext(context.Background()).ScanCAS(&conflictUsername2, &conflictAvatar2, &conflictID2)

	if err != nil {
		utils.Error("ScyllaDB LWT failed for users_by_username", "err", err)
		_ = db.Session.Query(`DELETE FROM users_by_email WHERE email = ?`, user.Email).Exec() // COMPENSATION
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur base de données. (Code: REG-011)"})
	}
	if !applied {
		_ = db.Session.Query(`DELETE FROM users_by_email WHERE email = ?`, user.Email).Exec() // COMPENSATION
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "Ce nom d'utilisateur est déjà utilisé. (Code: REG-008)"})
	}

	// --- ÉTAPE 3: Insérer l'utilisateur final ---
	if err := db.Session.Query(
		`INSERT INTO users (id, email, username, password, avatar, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Username, user.Password, user.Avatar, user.CreatedAt,
	).WithContext(context.Background()).Exec(); err != nil {
		utils.Error("ScyllaDB final user insert failed", "err", err)
		// COMPENSATION
		_ = db.Session.Query(`DELETE FROM users_by_email WHERE email = ?`, user.Email).Exec()
		_ = db.Session.Query(`DELETE FROM users_by_username WHERE username = ?`, user.Username).Exec()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur finale de création de compte. (Code: REG-012)"})
	}

	_ = utils.RedisDel(key)

	ua := c.Get("User-Agent")
	accept := c.Get("Accept")
	lang := c.Get("Accept-Language")
	encoding := c.Get("Accept-Encoding")
	ip := c.IP()
	deviceID := utils.GenerateDeviceID(ua, accept, lang, encoding, ip)

	accessToken, err := utils.GenerateAccessToken(scyllaUUID.String(), deviceID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur lors de la génération des token de connexion. (Code: REG-012)")
	}
	refreshToken, err := utils.GenerateRefreshToken(scyllaUUID.String(), deviceID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur lors de la génération des token de connexion. (Code: REG-013)")
	}
	response := fiber.Map{
		"message":       "🎉 Compte créé avec succès",
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}

	if cfg.Debug {
		response["device_id"] = deviceID
	}

	return c.Status(201).JSON(response)
}

func generateDefaultAvatarURL(username string) string {
	colors := []string{
		"0D8ABC", "F44336", "4CAF50", "FF9800", "9C27B0",
		"3F51B5", "795548", "607D8B", "009688", "E91E63",
	}

	// Choisir une couleur aléatoire
	rand.Seed(time.Now().UnixNano())
	color := colors[rand.Intn(len(colors))]

	// Capitaliser le nom (facultatif)
	name := strings.Title(strings.ReplaceAll(username, "_", " "))

	// Encoder le nom pour l’URL
	escapedName := url.QueryEscape(name)

	// Générer l’URL finale
	return fmt.Sprintf("https://ui-avatars.com/api/?background=%s&color=fff&name=%s", color, escapedName)
}
