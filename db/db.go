package db

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db/migration"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"strconv"
	"time"

	"github.com/gocql/gocql"
)

var Session *gocql.Session

func ConnectDB() {
	cfg := config.GetConfig()
	cluster := gocql.NewCluster(cfg.ScyllaHost)
	cluster.Port = getPort(cfg.ScyllaPort)
	cluster.Keyspace = cfg.ScyllaKeyspace
	cluster.Consistency = gocql.Quorum

	// Authentification activée
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: cfg.ScyllaUser,
		Password: cfg.ScyllaPass,
	}

	session, err := cluster.CreateSession()
	if err != nil {
		utils.Fatal("ScyllaDB connection failed", "error", err)
	}

	Session = session
	utils.Success("ScyllaDB connected successfully.")
}

func getPort(portStr string) int {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		utils.Fatal("Invalid port", "error", err)
	}
	return port
}

func ApplyMigrations(session *gocql.Session) {
	for _, m := range migration.AllMigrations {
		// Check si migration déjà faite
		var name string
		if err := session.Query(`SELECT name FROM migrations_applied WHERE name = ? LIMIT 1`, m.Name()).Scan(&name); err == nil {
			utils.Info("⏭️ Migration already applied", "name", m.Name())
			continue
		}

		utils.Info("⏳ Applying migration", "name", m.Name())
		if err := m.Up(session); err != nil {
			utils.Fatal("Migration failed", "name", m.Name(), "error", err)
		}

		// On log la migration comme faite
		if err := session.Query(
			`INSERT INTO migrations_applied (name, applied_at) VALUES (?, ?)`,
			m.Name(), time.Now(),
		).Exec(); err != nil {
			utils.Fatal("Failed to record migration", "name", m.Name(), "error", err)
		}

		utils.Success("Migration applied", "name", m.Name())
	}
}
