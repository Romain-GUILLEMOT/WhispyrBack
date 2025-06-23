package dbTools

import (
	"context"
	"fmt" // Importe le package fmt pour fmt.Errorf
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/db"    // Importe votre instance de session ScyllaDB
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils" // Pour les logs (utils.Error, utils.Info)
	"github.com/gocql/gocql"                        // Importe le package gocql pour son type UUID (nécessaire pour la conversion)
	"github.com/google/uuid"                        // Importe le package google/uuid (pour le parsing initial)
)

// SaveMessageToScylla enregistre un message dans la table messages_by_channel.
// Il utilise le `day_bucket` et le `sent_at` (TIMEUUID) pour optimiser les requêtes.
func SaveMessageToScylla(ctx context.Context, serverIDStr string, userIDStr string, content string, timestamp int64) error {
	// Conversion des IDs string en UUIDs Go (type github.com/google/uuid.UUID)
	googleServerID, err := uuid.Parse(serverIDStr)
	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Invalid server ID format for '%s': %v", serverIDStr, err))
		return fmt.Errorf("invalid server ID format: %w", err)
	}
	googleSenderID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Invalid user ID format for '%s': %v", userIDStr, err))
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// CONVERSION CRUCIALE : Convertir github.com/google/uuid.UUID en github.com/gocql/gocql.UUID
	gocqlServerID := gocql.UUID(googleServerID)
	gocqlSenderID := gocql.UUID(googleSenderID)

	// Conversion du timestamp Unix (millisecondes) en temps Go
	sentAtTime := time.Unix(0, timestamp*int64(time.Millisecond))

	// Création du TIMEUUID pour 'sent_at' (gocql.UUID est déjà le bon type ici)
	sentAtUUID := gocql.UUIDFromTime(sentAtTime)

	// Création du day_bucket (date sans l'heure). gocql gérera la conversion vers le type CQL DATE.
	dayBucket := sentAtTime

	// La requête d'insertion.
	// La table messages_by_channel utilise channel_id. Pour le moment, nous utilisons serverID
	// comme channel_id, car vous avez dit "un chat par serveurs sans prendre en compte les channels".
	// Lorsque vous implémenterez les channels, vous devrez passer l'ID du channel réel ici.
	query := `INSERT INTO messages_by_channel (channel_id, day_bucket, sent_at, sender_id, content) VALUES (?, ?, ?, ?, ?)`

	// Exécution de la requête en utilisant db.Session
	err = db.Session.Query(query,
		gocqlServerID, // Utilisation de gocql.UUID converti
		dayBucket,
		sentAtUUID,
		gocqlSenderID, // Utilisation de gocql.UUID converti
		content,
	).WithContext(ctx).Exec()

	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Failed to insert message into ScyllaDB for server %s by user %s: %v", serverIDStr, userIDStr, err))
		return fmt.Errorf("failed to insert message into ScyllaDB: %w", err)
	}

	utils.Info("Message saved to ScyllaDB for server " + serverIDStr + " by user " + userIDStr)
	return nil
}
