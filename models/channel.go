package models

import (
	"github.com/gocql/gocql"
	"time"
)

type Channel struct {
	ChannelID  gocql.UUID `json:"channel_id" validate:"required"`
	ServerID   gocql.UUID `json:"server_id"`
	CategoryID gocql.UUID `json:"category_id"`
	Name       string     `json:"name" validate:"required"`
	Type       string     `json:"type" validate:"required,oneof=text voice dm"`
	IsPrivate  bool       `json:"is_private"`
	Position   int        `json:"position"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ChannelByServer struct {
	ServerID   gocql.UUID `json:"server_id" validate:"required"`
	CategoryID gocql.UUID `json:"category_id"`
	Position   int        `json:"position"`
	ChannelID  gocql.UUID `json:"channel_id" validate:"required"`
	Name       string     `json:"name" validate:"required"`
	Type       string     `json:"type" validate:"required,oneof=text voice dm"`
	IsPrivate  bool       `json:"is_private"`
}
