package migration

import "github.com/gocql/gocql"

type FirstMigration struct{}

func (m FirstMigration) Name() string {
	return "12_05_2025_First_Migration"
}

func (m FirstMigration) Up(session *gocql.Session) error {
	cql := []string{
		// Migrations appliquées
		`CREATE TABLE IF NOT EXISTS migrations_applied (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMP
		);`,

		// Utilisateurs
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email TEXT,
			username TEXT,
			password TEXT,
			avatar TEXT,
			channels_accessible LIST<UUID>,
			created_at TIMESTAMP
		);`,

		// Channels (public ou privés)
		`CREATE TABLE IF NOT EXISTS channels (
			id UUID PRIMARY KEY,
			name TEXT,
			is_private BOOLEAN,
			creator_id UUID,
			created_at TIMESTAMP
		);`,

		// Messages (par channel)
		`CREATE TABLE IF NOT EXISTS messages_by_channel (
			channel_id UUID,
			sent_at TIMEUUID,
			sender_id UUID,
			content TEXT,
			PRIMARY KEY (channel_id, sent_at)
		);`,
		`CREATE INDEX IF NOT EXISTS email_idx ON users (email)`,
	}

	for _, query := range cql {
		if err := session.Query(query).Exec(); err != nil {
			return err
		}
	}
	return nil
}
