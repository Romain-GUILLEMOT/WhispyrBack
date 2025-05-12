package auth

import (
	"context"
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/htmlemail"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"math/rand"
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

	var existing string
	err = db.Session.Query(`SELECT email FROM users WHERE email = ? LIMIT 1`, email).Scan(&existing)
	if err == nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Un compte avec cet email existe déjà. (Code: WHIAUTH-004)",
		})
	}

	code := fmt.Sprintf("%04d", rand.Intn(10000))
	key := "email_verif:" + email
	ttl, err := utils.RedisTTL(key)
	if cfg.Debug {
		utils.Info("TTL", "ttl", ttl)
		if cfg.Debug {
			utils.RedisDel(key)
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

	if err := validate.Struct(input); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Champs invalides. (Code: WHIREG-001)")
	}

	key := "acc_reg:" + email
	cfg := config.GetConfig()
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
	file, err := c.FormFile("avatar")
	if err == nil {
		src, _ := file.Open()
		defer src.Close()
		converted, err := utils.ConvertToWebP(src, file.Header.Get("Content-Type"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Erreur conversion WebP. (Code: REG-004)")
		}
		randStr, _ := utils.RandomString64()
		name := randStr + ".webp"
		_, err = utils.MinioClient.PutObject(context.Background(), "photos", name, converted, int64(converted.Len()), minio.PutObjectOptions{
			ContentType: "image/webp",
		})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Erreur upload MinIO. (Code: REG-005)")
		}
		avatarURL = fmt.Sprintf("https://whyspir-cdn.yotapaki.dev/photos/%s", name)
	}

	// Hash du mot de passe
	hashedPass, err := utils.HashPassword(input.Password)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Erreur de sécurité. (Code: REG-006)")
	}

	user := models.User{
		ID:                 uuid.New(),
		Username:           input.Username,
		Password:           hashedPass,
		Avatar:             avatarURL,
		ChannelsAccessible: []uuid.UUID{},
		CreatedAt:          time.Now(),
	}

	// Insert dans ScyllaDB
	if err := db.Session.Query(`
		INSERT INTO users (
			id, username, password, avatar, channels_accessible, created_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Password, user.Avatar, user.ChannelsAccessible, user.CreatedAt,
	).Exec(); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Erreur lors de l’insertion. (Code: REG-007)",
			"error": func() string {
				if cfg.Debug {
					return err.Error()
				}
				return "Unknown"
			}(),
		})
	}

	_ = utils.RedisDel(key)

	return c.Status(201).JSON(fiber.Map{
		"message": "🎉 Compte créé avec succès",
		"user_id": user.ID,
	})
}
