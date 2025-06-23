package models

import "github.com/gocql/gocql"

type PrivateChannelByUser struct {
	UserID    gocql.UUID `json:"user_id" validate:"required"`
	ChannelID gocql.UUID `json:"channel_id" validate:"required"`
	LastMsgAt gocql.UUID `json:"last_msg_at" validate:"required"`
}
