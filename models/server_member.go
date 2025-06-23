package models

import (
	"github.com/gocql/gocql"
	"time"
)

type UserServer struct {
	UserID   gocql.UUID `json:"user_id" validate:"required"`
	ServerID gocql.UUID `json:"server_id" validate:"required"`
	Role     string     `json:"role" validate:"required"`
	JoinedAt time.Time  `json:"joined_at"`
}

type ServerMember struct {
	ServerID gocql.UUID `json:"server_id" validate:"required"`
	UserID   gocql.UUID `json:"user_id" validate:"required"`
	Role     string     `json:"role" validate:"required"`
	JoinedAt time.Time  `json:"joined_at"`
	Username string     `json:"username" validate:"required"`
	Avatar   string     `json:"avatar" validate:"omitempty,url"`
}
