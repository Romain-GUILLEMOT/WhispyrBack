package dbTools

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/gocql/gocql"
)

// GetServerByID récupère un serveur et son nom par son ID.
func GetServerByID(id string) (*models.Server, error) {
	var server models.Server

	// Convertit l'ID string du message en gocql.UUID pour la requête
	parsedID, err := gocql.ParseUUID(id)
	if err != nil {
		return nil, err
	}

	query := `SELECT server_id, name FROM servers WHERE server_id = ? LIMIT 1`

	if err := db.Session.Query(query, parsedID).Scan(
		&server.ServerID,
		&server.Name,
	); err != nil {
		return nil, err
	}

	return &server, nil
}
