package models

import (
	"github.com/gocql/gocql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                 gocql.UUID  `json:"id" validate:"required,uuid4"`
	Email              string      `json:"email" validate:"required,email"`
	Username           string      `json:"username" validate:"required,min=3,max=32"`
	Password           string      `json:"-" validate:"required,min=8"`
	Avatar             string      `json:"avatar" validate:"omitempty,url"`
	ChannelsAccessible []uuid.UUID `json:"channels_accessible" validate:"dive,uuid4"`
	CreatedAt          time.Time   `json:"created_at"`
}
