package dbTools

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
)

func GetUserByID(id *uuid.UUID) (*models.User, error) {
	var user models.User

	// Convertir uuid.UUID → gocql.UUID
	gocqlID := gocql.UUID(*id)

	query := `
		SELECT username, email, avatar
		FROM users WHERE id = ? LIMIT 1
	`

	if err := db.Session.Query(query, gocqlID).Scan(
		&user.Username,
		&user.Email,
		&user.Avatar,
	); err != nil {
		return nil, err
	}

	return &user, nil
}
