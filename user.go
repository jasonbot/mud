package mud

import (
	"encoding/json"
	"log"

	bolt "github.com/coreos/bbolt"
)

// User represents an active user in the system.
type User interface {
	Username() string
	Location() *Point
	Reload()
	Save()
}

// UserSSHAuthentication for storing SSH auth.
type UserSSHAuthentication interface {
	SSHKeysEmpty() bool
	ValidateSSHKey(string) bool
	AddSSHKey(string)
}

// UserData is a JSON-serializable set of information about a User.
type UserData struct {
	Username    string          `json:""`
	X           uint32          `json:""`
	Y           uint32          `json:""`
	Initialized bool            `json:""`
	PublicKeys  map[string]bool `json:""`
}

type dbUser struct {
	UserData
	world *dbWorld
}

func (user *dbUser) Username() string {
	return user.UserData.Username
}

func (user *dbUser) Location() *Point {
	return &Point{X: user.X, Y: user.Y}
}

func (user *dbUser) Reload() {
	var record []byte
	user.world.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))

		record = bucket.Get([]byte(user.UserData.Username))

		return nil
	})

	if record == nil {
		log.Printf("User %s does not exist, creating anew...", user.UserData.Username)
		user.UserData = user.world.newUser(user.UserData.Username)
	} else {
		json.Unmarshal(record, &(user.UserData))
	}
}

func (user *dbUser) Save() {
	bytes, err := json.Marshal(user.UserData)
	if err != nil {
		log.Printf("Can't marshal user: %v", err)
		return
	}

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))

		err = bucket.Put([]byte(user.UserData.Username), bytes)

		return err
	})
}

func (user *dbUser) SSHKeysEmpty() bool {
	return len(user.UserData.PublicKeys) == 0
}

func (user *dbUser) ValidateSSHKey(sshKey string) bool {
	val, ok := user.UserData.PublicKeys[sshKey]
	return val && ok
}

func (user *dbUser) AddSSHKey(sshKey string) {
	user.UserData.PublicKeys[sshKey] = true
	user.Save()
}

func getUserFromDB(world *dbWorld, username string) User {
	log.Printf("Looking up %s", username)
	user := dbUser{UserData: UserData{
		Username: username},
		world: world}

	user.Reload()

	return &user
}
