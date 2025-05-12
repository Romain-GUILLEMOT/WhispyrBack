package models

import (
	"time"

	"github.com/google/uuid"
)

type Channel struct {
	ID        uuid.UUID `json:"id" validate:"required,uuid4"`
	Name      string    `json:"name" validate:"required,min=3,max=100"`
	IsPrivate bool      `json:"is_private"`
	CreatorID uuid.UUID `json:"creator_id" validate:"required,uuid4"`
	CreatedAt time.Time `json:"created_at"`
}
