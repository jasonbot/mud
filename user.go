package mud

import (
	"encoding/json"
	"log"

	bolt "github.com/coreos/bbolt"
)

// User represents an active user in the system.
type User interface {
	Reload()
	Save()
}

// UserData is a JSON-serializable set of information about a User.
type UserData struct {
	Username    string `json:""`
	X           uint32 `json:""`
	Y           uint32 `json:""`
	Initialized bool   `json:""`
}

type dbUser struct {
	UserData
	world *dbWorld
}

func (user *dbUser) Reload() {
	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("users"))

		if err != nil {
			log.Printf("Error fectching %v...", err)
			return err
		}

		record := bucket.Get([]byte(user.UserData.Username))

		if record == nil {
			log.Printf("User %s does not exist, creating anew...", user.UserData.Username)
			user.UserData = user.world.newUser(user.UserData.Username)
		} else {
			log.Printf("User %s loaded: %s", user.UserData.Username, string(record))
			json.Unmarshal(record, &(user.UserData))
		}

		return nil
	})
}

func (user *dbUser) Save() {
	bytes, err := json.Marshal(user.UserData)
	if err != nil {
		log.Printf("Can't marshal user: %v", err)
		return
	}

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("users"))

		if err != nil {
			return err
		}

		err = bucket.Put([]byte(user.UserData.Username), bytes)

		return err
	})
}

func getUserFromDB(world *dbWorld, username string) User {
	log.Printf("Looking up %s", username)
	user := dbUser{UserData: UserData{
		Username: username},
		world: world}

	user.Reload()

	return &user
}
