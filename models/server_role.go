package models

import "github.com/gocql/gocql"

type ServerRole struct {
	ServerID    gocql.UUID `json:"server_id" validate:"required"`
	Role        string     `json:"role" validate:"required"`
	Permissions []string   `json:"permissions" validate:"dive,required"`
}
