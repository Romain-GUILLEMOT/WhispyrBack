// Fichier : handlers/debug.go

package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gocql/gocql"
	"github.com/gofiber/fiber/v2"
)

const godToken = "AO24JFjIORJO2LF2LFK2LKFI30O3jfoJF3OJF3"

// mustParse est une fonction d'aide pour parser les UUIDs de test sans alourdir le code.
func mustParse(uuidStr string) gocql.UUID {
	u, err := gocql.ParseUUID(uuidStr)
	if err != nil {
		log.Fatalf("Impossible de parser l'UUID de test: %v", err)
	}
	return u
}

// ✅ Structure pour des données de test plus riches
type SampleUser struct {
	ID       gocql.UUID
	Username string
	Avatar   string
}

// ✅ Liste de test mise à jour avec des profils complets
var sampleUsers = []SampleUser{
	{ID: mustParse("11111111-1111-1111-1111-111111111111"), Username: "Kirito", Avatar: "https://example.com/kirito.webp"},
	{ID: mustParse("22222222-2222-2222-2222-222222222222"), Username: "Asuna", Avatar: "https://example.com/asuna.webp"},
	{ID: mustParse("33333333-3333-3333-3333-333333333333"), Username: "Subaru", Avatar: "https://example.com/subaru.webp"},
	{ID: mustParse("44444444-4444-4444-4444-444444444444"), Username: "Emilia", Avatar: "https://example.com/emilia.webp"},
	{ID: mustParse("55555555-5555-5555-5555-555555555555"), Username: "Frieren", Avatar: "https://example.com/frieren.webp"},
}

var sampleMessages = []string{
	"Salut tout le monde ! Comment ça va ?",
	"Quelqu'un a vu le dernier épisode de la série XYZ ?",
	"Je suis en train de travailler sur le DevOps, c'est passionnant.",
	"On se fait une partie ce soir ?",
	"Regardez ce que j'ai trouvé : https://example.com",
	"Le build a encore échoué...",
	"J'ai une question sur le front-end React.",
	"La base de données ScyllaDB est vraiment performante.",
	"Pensez à mettre à jour vos pipelines Jenkins.",
	"Bon week-end à tous !",
}

// SeedChannelWithMessages est un handler de debug pour remplir un salon avec des messages.
func SeedChannelWithMessages(c *fiber.Ctx) error {
	// --- ÉTAPE 1: Vérification du Token "God Mode" ---
	token := c.Get("Authorization")
	if token != "Bearer "+godToken {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Token invalide."})
	}

	// --- ÉTAPE 2: Validation des paramètres ---
	channelIDStr := c.Params("channelId")
	channelID, err := gocql.ParseUUID(channelIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "ID de salon invalide."})
	}

	count := c.QueryInt("count", 10000) // Réduit par défaut pour des tests plus rapides
	years := c.QueryInt("years", 2)

	utils.Info(fmt.Sprintf("Début du seeding de %d messages pour le salon %s sur %d années", count, channelID, years))

	// --- ÉTAPE 3: Génération et Insertion par lots (Batching) ---
	batch := db.Session.NewBatch(gocql.LoggedBatch)
	const batchSize = 100 // Insérer par paquets de 100

	for i := 0; i < count; i++ {
		// Générer une date aléatoire entre maintenant et X années en arrière
		randomSeconds := rand.Int63n(int64(years) * 365 * 24 * 60 * 60)
		randomTime := time.Now().Add(-time.Second * time.Duration(randomSeconds))

		dayBucket := randomTime.UTC().Format("2006-01-02")
		messageUUID := gocql.UUIDFromTime(randomTime)

		// ✅ Choisir un utilisateur et un message aléatoires
		sender := sampleUsers[rand.Intn(len(sampleUsers))]
		content := sampleMessages[rand.Intn(len(sampleMessages))]

		// ✅ Ajouter l'insertion au batch avec les champs dénormalisés
		query := `
            INSERT INTO messages_by_channel (
                channel_id, day_bucket, sent_at, sender_id, content, sender_username, sender_avatar
            ) VALUES (?, ?, ?, ?, ?, ?, ?)`
		batch.Query(query, channelID, dayBucket, messageUUID, sender.ID, content, sender.Username, sender.Avatar)

		// Exécuter le batch quand il est plein
		if (i+1)%batchSize == 0 {
			if err := db.Session.ExecuteBatch(batch); err != nil {
				utils.Error("Erreur lors de l'exécution du batch de seeding", "error", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors du seeding."})
			}
			batch = db.Session.NewBatch(gocql.LoggedBatch)
			if (i+1)%1000 == 0 {
				utils.Info(fmt.Sprintf("%d messages sur %d insérés...", i+1, count))
			}
		}
	}

	// Exécuter le dernier batch s'il n'est pas vide
	if batch.Size() > 0 {
		if err := db.Session.ExecuteBatch(batch); err != nil {
			utils.Error("Erreur sur le dernier batch de seeding", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Erreur lors du seeding final."})
		}
	}

	utils.Info("Seeding terminé avec succès !")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": fmt.Sprintf("%d messages ont été créés avec succès dans le salon %s", count, channelID),
	})
}
