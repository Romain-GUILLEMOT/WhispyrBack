package migration

import "github.com/gocql/gocql"

// ThirdMigration dénormalise le pseudo et l'avatar de l'expéditeur
// directement dans la table des messages pour optimiser les lectures.
type ThirdMigration struct{}

// Name retourne un nom unique pour cette migration.
func (m ThirdMigration) Name() string {
	return "07_07_2025_Denormalize_Message_Sender"
}

// Up exécute la commande CQL pour appliquer la migration.
func (m ThirdMigration) Up(session *gocql.Session) error {
	// On ajoute les deux colonnes à la table messages_by_channel.
	// L'objectif est d'éviter une seconde requête (un "join") à la lecture.
	// On écrit un peu plus de données une fois, pour lire beaucoup plus vite, des millions de fois.
	cqlCommands := []string{
		`ALTER TABLE messages_by_channel ADD sender_username TEXT;`,
		`ALTER TABLE messages_by_channel ADD sender_avatar TEXT;`,
	}

	for _, command := range cqlCommands {
		if err := session.Query(command).Exec(); err != nil {
			return err
		}
	}

	return nil
}
