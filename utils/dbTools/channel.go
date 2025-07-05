package dbTools

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/gocql/gocql"
	"time"
)

// GetChannelByID récupère un salon et son nom par son ID.
func GetChannelByID(id string) (*models.Channel, error) {
	var channel models.Channel

	// Convertit l'ID string du message en gocql.UUID pour la requête
	parsedID, err := gocql.ParseUUID(id)
	if err != nil {
		return nil, err
	}

	query := `SELECT channel_id, name FROM channels WHERE channel_id = ? LIMIT 1`

	if err := db.Session.Query(query, parsedID).Scan(
		&channel.ChannelID,
		&channel.Name,
	); err != nil {
		return nil, err
	}

	return &channel, nil
}

// CreateChannelInDB insère un nouveau salon dans toutes les tables nécessaires.
func CreateChannelInDB(serverIDStr, categoryIDStr, name, channelType string) error {
	serverID, _ := gocql.ParseUUID(serverIDStr)
	categoryID, _ := gocql.ParseUUID(categoryIDStr)
	channelID := gocql.TimeUUID()

	// TODO: Calculer la position dynamiquement
	position := 1

	batch := db.Session.NewBatch(gocql.LoggedBatch)
	batch.Query(`INSERT INTO channels (channel_id, server_id, category_id, name, type, is_private, position, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		channelID, serverID, categoryID, name, channelType, false, position, time.Now())

	batch.Query(`INSERT INTO channels_by_server (server_id, category_id, position, channel_id, name, type, is_private) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		serverID, categoryID, position, channelID, name, channelType, false)

	return db.Session.ExecuteBatch(batch)
}

// UpdateChannelInDB met à jour le nom d'un salon.
func UpdateChannelInDB(channelIDStr, newName string) error {
	channelID, _ := gocql.ParseUUID(channelIDStr)

	// Il faut d'abord lire pour avoir les clés primaires des autres tables
	var serverID, categoryID gocql.UUID
	var position int
	if err := db.Session.Query(`SELECT server_id, category_id, position FROM channels WHERE channel_id = ?`, channelID).Scan(&serverID, &categoryID, &position); err != nil {
		return err
	}

	batch := db.Session.NewBatch(gocql.LoggedBatch)
	batch.Query(`UPDATE channels SET name = ? WHERE channel_id = ?`, newName, channelID)
	batch.Query(`UPDATE channels_by_server SET name = ? WHERE server_id = ? AND category_id = ? AND position = ?`, newName, serverID, categoryID, position)

	return db.Session.ExecuteBatch(batch)
}

// DeleteChannelFromDB supprime un salon de toutes les tables.
func DeleteChannelFromDB(channelIDStr string) error {
	channelID, _ := gocql.ParseUUID(channelIDStr)

	var serverID, categoryID gocql.UUID
	var position int
	if err := db.Session.Query(`SELECT server_id, category_id, position FROM channels WHERE channel_id = ?`, channelID).Scan(&serverID, &categoryID, &position); err != nil {
		return err
	}

	batch := db.Session.NewBatch(gocql.LoggedBatch)
	batch.Query(`DELETE FROM channels WHERE channel_id = ?`, channelID)
	batch.Query(`DELETE FROM channels_by_server WHERE server_id = ? AND category_id = ? AND position = ?`, serverID, categoryID, position)
	// IMPORTANT : Il faudra aussi supprimer les messages de ce salon
	// batch.Query(`DELETE FROM messages_by_channel WHERE channel_id = ?`, channelID)

	return db.Session.ExecuteBatch(batch)
}
