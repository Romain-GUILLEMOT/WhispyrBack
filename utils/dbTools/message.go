// Dans un fichier comme dbTools/scylla_actions.go

package dbTools

import (
	"context"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gocql/gocql"
)

// SaveMessageToScylla enregistre un message de chat dans la base de données,
// incluant les informations dénormalisées de l'expéditeur (pseudo et avatar).
// La fonction génère son propre timestamp pour être la source de vérité.
func SaveMessageToScylla(ctx context.Context, serverID, channelID, userID, content, senderUsername, senderAvatar string) {
	channelUUID, err := gocql.ParseUUID(channelID)
	if err != nil {
		utils.Error("SaveMessageToScylla: ID de salon invalide", "channelId", channelID, "error", err)
		return
	}
	senderUUID, err := gocql.ParseUUID(userID)
	if err != nil {
		utils.Error("SaveMessageToScylla: ID utilisateur invalide", "userId", userID, "error", err)
		return
	}

	now := time.Now()
	dayBucket := now.UTC().Format("2006-01-02")
	messageUUID := gocql.UUIDFromTime(now)

	// ✅ La requête INSERT inclut maintenant sender_username et sender_avatar
	query := `
        INSERT INTO messages_by_channel (
            channel_id, day_bucket, sent_at, sender_id, content, sender_username, sender_avatar
        ) VALUES (?, ?, ?, ?, ?, ?, ?)`

	if err := db.Session.Query(
		query,
		channelUUID,
		dayBucket,
		messageUUID,
		senderUUID,
		content,
		senderUsername, // On ajoute le pseudo
		senderAvatar,   // et l'avatar
	).WithContext(ctx).Exec(); err != nil {
		utils.Error("Erreur lors de la sauvegarde du message dans ScyllaDB", "error", err)
	}
}
