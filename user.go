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
	username    string `json:""`
	x           uint32 `json:""`
	y           uint32 `json:""`
	initialized bool   `json:""`
}

type dbUser struct {
	UserData
	world *dbWorld
}

func (user *dbUser) Reload() {
	user.world.database.View(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("users"))

		if err != nil {
			return err
		}

		record := bucket.Get([]byte(user.UserData.username))

		if record == nil {
			user.UserData = user.world.newUser(user.UserData.username)
		} else {
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

		err = bucket.Put([]byte(user.UserData.username), bytes)

		return err
	})
}

func getUserFromDB(world *dbWorld, username string) User {
	user := dbUser{UserData: UserData{
		username: username},
		world: world}

	user.Reload()

	return &user
}
