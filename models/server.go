package models

import (
	"github.com/gocql/gocql"
	"time"
)

type Server struct {
	ServerID  gocql.UUID `json:"server_id" validate:"required"`
	Name      string     `json:"name" validate:"required"`
	OwnerID   gocql.UUID `json:"owner_id" validate:"required"`
	CreatedAt time.Time  `json:"created_at"`
}
