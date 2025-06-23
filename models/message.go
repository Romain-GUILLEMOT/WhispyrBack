package models

import (
	"github.com/gocql/gocql"
	"time"
)

type MessageByChannel struct {
	ChannelID gocql.UUID `json:"channel_id" validate:"required"`
	DayBucket time.Time  `json:"day_bucket" validate:"required"` // YYYY-MM-DD
	SentAt    gocql.UUID `json:"sent_at" validate:"required"`
	SenderID  gocql.UUID `json:"sender_id" validate:"required"`
	Content   string     `json:"content" validate:"required"`
}
