package dbTools

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/models"
	"github.com/google/uuid"
)

func GetUserByID(id *uuid.UUID) (*models.User, error) {
	var user models.User

	query := `
		SELECT id, username, password, avatar, channels_accessible, created_at
		FROM users WHERE id = ? LIMIT 1
	`

	if err := db.Session.Query(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Avatar,
		&user.ChannelsAccessible,
		&user.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &user, nil
}
