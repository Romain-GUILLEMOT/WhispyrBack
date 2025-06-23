package models

import "github.com/gocql/gocql"

type UserByEmail struct {
	Email    string     `json:"email" validate:"required,email"`
	ID       gocql.UUID `json:"id" validate:"required"`
	Username string     `json:"username" validate:"required"`
	Avatar   string     `json:"avatar" validate:"omitempty,url"`
}

type UserByUsername struct {
	Username string     `json:"username" validate:"required"`
	ID       gocql.UUID `json:"id" validate:"required"`
	Avatar   string     `json:"avatar" validate:"omitempty,url"`
}
