package migration

import "github.com/gocql/gocql"

type Migration interface {
	Name() string
	Up(session *gocql.Session) error
}
