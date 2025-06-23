package models

import (
	"github.com/gocql/gocql"
	"time"
)

type User struct {
	ID        gocql.UUID `json:"id" validate:"required"`
	Email     string     `json:"email" validate:"required,email"`
	Username  string     `json:"username" validate:"required,min=3,max=32"`
	Password  string     `json:"-" validate:"required,min=8"`
	Avatar    string     `json:"avatar" validate:"omitempty,url"`
	CreatedAt time.Time  `json:"created_at"`
}
