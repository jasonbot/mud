package mud

import bolt "github.com/coreos/bbolt"

// User represents an active user in the system.
type User interface {
}

type dbUser struct {
	username string
	database *bolt.DB
}

func getUserFromDB(database *bolt.DB, username string) User {
	user := dbUser{username: username, database: database}

	return user
}
