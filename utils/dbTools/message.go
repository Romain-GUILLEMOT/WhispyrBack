package dbTools

import (
	"context"
	"fmt"
	"time"

	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

// MODIFIÉ : La fonction accepte maintenant channelIDStr en plus.
func SaveMessageToScylla(ctx context.Context, serverIDStr string, channelIDStr string, userIDStr string, content string, timestamp int64) error {
	// Conversion des IDs string en UUIDs Go
	googleChannelID, err := uuid.Parse(channelIDStr)
	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Invalid channel ID format for '%s': %v", channelIDStr, err))
		return fmt.Errorf("invalid channel ID format: %w", err)
	}
	googleSenderID, err := uuid.Parse(userIDStr)
	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Invalid user ID format for '%s': %v", userIDStr, err))
		return fmt.Errorf("invalid user ID format: %w", err)
	}

	// Conversion des UUIDs Go en UUIDs gocql
	gocqlChannelID := gocql.UUID(googleChannelID)
	gocqlSenderID := gocql.UUID(googleSenderID)

	// Conversion du timestamp et création du TIMEUUID
	sentAtTime := time.Unix(0, timestamp*int64(time.Millisecond))
	sentAtUUID := gocql.UUIDFromTime(sentAtTime)
	dayBucket := sentAtTime // Si day_bucket est de type TIMESTAMP ou DATE dans Scylla, utilisez time.Time

	// MODIFIÉ : La requête utilise maintenant le vrai channel_id
	query := `INSERT INTO messages_by_channel (channel_id, day_bucket, sent_at, sender_id, content) VALUES (?, ?, ?, ?, ?)`

	err = db.Session.Query(query,
		gocqlChannelID, // Utilisation de l'ID du salon
		dayBucket,
		sentAtUUID,
		gocqlSenderID,
		content,
	).WithContext(ctx).Exec()

	if err != nil {
		utils.Error(fmt.Sprintf("SaveMessageToScylla: Failed to insert message into ScyllaDB for channel %s by user %s: %v", channelIDStr, userIDStr, err))
		return fmt.Errorf("failed to insert message into ScyllaDB: %w", err)
	}

	utils.Info(fmt.Sprintf("Message saved to ScyllaDB for channel %s (server %s) by user %s", channelIDStr, serverIDStr, userIDStr))
	return nil
}

// NOTE : Vous devrez peut-être ajouter la fonction GetServerByID et GetUserByID
// ou vous assurer que leur implémentation est disponible dans ce package ou ailleurs.
// Exemple (à adapter selon votre structure de DB et vos modèles) :
/*
import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/models" // Assurez-vous que le chemin est correct pour vos modèles
	"github.com/gocql/gocql"
)

func GetUserByID(userID *uuid.UUID) (*models.User, error) {
    var user models.User
    query := "SELECT id, username, avatar FROM users WHERE id = ?"
    gocqlUserID := gocql.UUID(*userID)
    if err := db.Session.Query(query, gocqlUserID).Scan(&user.UserID, &user.Username, &user.Avatar); err != nil {
        if err == gocql.ErrNotFound {
            return nil, fmt.Errorf("user not found: %s", userID.String())
        }
        return nil, fmt.Errorf("error getting user by ID %s: %w", userID.String(), err)
    }
    return &user, nil
}

func GetServerByID(serverID string) (*models.Server, error) {
    var server models.Server
    serverUUID, err := uuid.Parse(serverID)
    if err != nil {
        return nil, fmt.Errorf("invalid server ID format: %w", err)
    }
    gocqlServerID := gocql.UUID(serverUUID)

    query := "SELECT id, name FROM servers WHERE id = ?"
    if err := db.Session.Query(query, gocqlServerID).Scan(&server.ServerID, &server.Name); err != nil {
        if err == gocql.ErrNotFound {
            return nil, fmt.Errorf("server not found: %s", serverID)
        }
        return nil, fmt.Errorf("error getting server by ID %s: %w", serverID, err)
    }
    return &server, nil
}
*/
