package migration

import "github.com/gocql/gocql"

type FirstMigration struct{}

func (m FirstMigration) Name() string {
	return "12_05_2025_First_Migration"
}

func (m FirstMigration) Up(session *gocql.Session) error {
	cql := []string{
		// ------------------------------------------------------------
		// 0. Journal des migrations
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS migrations_applied (
            name TEXT PRIMARY KEY,
            applied_at TIMESTAMP
        );`,

		// ------------------------------------------------------------
		// 1. Utilisateurs
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS users (
            id         UUID    PRIMARY KEY,
            email      TEXT,
            username   TEXT,
            password   TEXT,
            avatar     TEXT,
            created_at TIMESTAMP
        );`,

		// Tables de lookup « write-time »
		`CREATE TABLE IF NOT EXISTS users_by_email (
            email      TEXT,
            id         UUID,
            username   TEXT,
            avatar     TEXT,
            PRIMARY KEY ((email))
        );`,

		`CREATE TABLE IF NOT EXISTS users_by_username (
            username   TEXT,
            id         UUID,
            avatar     TEXT,
            PRIMARY KEY ((username))
        );`,

		// ------------------------------------------------------------
		// 2. Serveurs
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS servers (
            server_id  UUID    PRIMARY KEY,
            name       TEXT,
            owner_id   UUID,
            created_at TIMESTAMP
        );`,

		// ------------------------------------------------------------
		// 3. Liaison users ↔ servers
		// ------------------------------------------------------------
		// Vue côté utilisateur
		`CREATE TABLE IF NOT EXISTS user_servers (
            user_id    UUID,
            server_id  UUID,
            role       TEXT,
            joined_at  TIMESTAMP,
            PRIMARY KEY ((user_id), server_id)
        );`,

		// Vue miroir côté serveur (dénormalisée avec username + avatar)
		`CREATE TABLE IF NOT EXISTS server_members (
            server_id  UUID,
            user_id    UUID,
            role       TEXT,
            joined_at  TIMESTAMP,
            username   TEXT,
            avatar     TEXT,
            PRIMARY KEY ((server_id), user_id)
        );`,

		// ------------------------------------------------------------
		// 4. Rôles et permissions
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS server_roles (
            server_id   UUID,
            role        TEXT,
            permissions SET<TEXT>,
            PRIMARY KEY ((server_id), role)
        );`,

		// ------------------------------------------------------------
		// 5. Catégories & canaux
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS categories_by_server (
            server_id   UUID,
            category_id UUID,
            name        TEXT,
            position    INT,
            PRIMARY KEY ((server_id), category_id)
        );`,

		// Lookup direct par channel_id (DM/Groupes inclus)
		`CREATE TABLE IF NOT EXISTS channels (
            channel_id   UUID PRIMARY KEY,
            server_id    UUID,      /* NULL si DM/Groupe   */
            category_id  UUID,      /* NULL si DM/Groupe   */
            name         TEXT,
            type         TEXT,      /* "text"|"voice"|"dm" */
            is_private   BOOLEAN,
            position     INT,
            created_at   TIMESTAMP
        );`,

		// Liste ordonnée des canaux d’un serveur
		`CREATE TABLE IF NOT EXISTS channels_by_server (
            server_id    UUID,
            category_id  UUID,
            position     INT,
            channel_id   UUID,
            name         TEXT,
            type         TEXT,
            is_private   BOOLEAN,
            PRIMARY KEY ((server_id), category_id, position)
        ) WITH CLUSTERING ORDER BY (category_id ASC, position ASC);`,

		// ------------------------------------------------------------
		// 6. Membres de canaux privés (DM / groupes)
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS channel_members (
            channel_id UUID,
            user_id    UUID,
            joined_at  TIMESTAMP,
            PRIMARY KEY ((channel_id), user_id)
        );`,

		// ------------------------------------------------------------
		// 7. DM / groupes par utilisateur (sidebar récente)
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS private_channels_by_user (
            user_id      UUID,
            channel_id   UUID,
            last_msg_at  TIMEUUID,
            PRIMARY KEY ((user_id), last_msg_at, channel_id)
        ) WITH CLUSTERING ORDER BY (last_msg_at DESC);`,

		// ------------------------------------------------------------
		// 8. Messages (bucket journalier pour éviter les mega-partitions)
		// ------------------------------------------------------------
		`CREATE TABLE IF NOT EXISTS messages_by_channel (
            channel_id UUID,
            day_bucket DATE,           /* ex : 2025-05-16                 */
            sent_at    TIMEUUID,       /* ordre logique + horodatage précis*/
            sender_id  UUID,
            content    TEXT,
            PRIMARY KEY ((channel_id, day_bucket), sent_at)
        ) WITH CLUSTERING ORDER BY (sent_at DESC);`,
	}

	for _, q := range cql {
		if err := session.Query(q).Exec(); err != nil {
			return err
		}
	}
	return nil
}
