package migration

import "github.com/gocql/gocql"

// SecondMigration ajoute un champ 'avatar' à la table 'servers'
// et le dénormalise dans 'user_servers' pour des lectures optimisées.
type SecondMigration struct{}

// Name retourne un nom unique pour cette migration.
func (m SecondMigration) Name() string {
	// Utilise la date du jour pour un nom unique et chronologique
	return "23_06_2025_Add_Server_Avatar"
}

// Up exécute les commandes CQL pour appliquer la migration.
func (m SecondMigration) Up(session *gocql.Session) error {
	// On utilise ALTER TABLE pour ajouter la nouvelle colonne aux tables existantes.
	// C'est une opération "schema-altering" qui ne bloque pas les lectures/écritures.
	cqlCommands := []string{
		// ------------------------------------------------------------
		// 1. Ajoute la colonne 'avatar' à la table principale des serveurs.
		//    C'est la source de vérité pour l'avatar du serveur.
		// ------------------------------------------------------------
		`ALTER TABLE servers ADD avatar TEXT;`,

		// ------------------------------------------------------------
		// 2. Dénormalise 'server_avatar' dans la table de liaison `user_servers`.
		//    Le but est d'éviter une deuxième requête coûteuse (un JOIN implicite)
		//    lorsqu'on récupère la liste des serveurs pour un utilisateur.
		//    On aura directement l'URL de l'avatar avec la première requête.
		// ------------------------------------------------------------
		`ALTER TABLE user_servers ADD server_avatar TEXT;`,
	}

	for _, command := range cqlCommands {
		if err := session.Query(command).Exec(); err != nil {
			// Si une commande échoue, on retourne l'erreur pour arrêter le processus de migration.
			return err
		}
	}

	return nil
}
