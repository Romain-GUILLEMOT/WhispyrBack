package models

import (
	"github.com/gocql/gocql"
)

type CategoryByServer struct {
	ServerID   gocql.UUID `json:"server_id" validate:"required"`
	CategoryID gocql.UUID `json:"category_id" validate:"required"`
	Name       string     `json:"name" validate:"required"`
	Position   int        `json:"position"`
}
