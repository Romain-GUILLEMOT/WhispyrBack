package models

import (
	"github.com/gocql/gocql"
	"time"
)

type ChannelMember struct {
	ChannelID gocql.UUID `json:"channel_id" validate:"required"`
	UserID    gocql.UUID `json:"user_id" validate:"required"`
	JoinedAt  time.Time  `json:"joined_at"`
}
