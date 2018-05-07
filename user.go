package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"
	"time"

	bolt "github.com/coreos/bbolt"
)

// User represents an active user in the system.
type User interface {
	Username() string
	Location() *Point

	HP() uint64
	SetHP(uint64)
	MP() uint64
	SetMP(uint64)
	AP() uint64
	SetAP(uint64)
	MaxHP() uint64
	SetMaxHP(uint64)
	MaxMP() uint64
	SetMaxMP(uint64)
	MaxAP() uint64
	SetMaxAP(uint64)
	LocationName() string

	MoveNorth()
	MoveSouth()
	MoveEast()
	MoveWest()

	Log(message string)
	GetLog() []string

	MarkActive()

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
	SpawnX      uint32          `json:""`
	SpawnY      uint32          `json:""`
	HP          uint64          `json:""`
	MaxHP       uint64          `json:""`
	AP          uint64          `json:""`
	MaxAP       uint64          `json:""`
	MP          uint64          `json:""`
	MaxMP       uint64          `json:""`
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

func (user *dbUser) HP() uint64 {
	return user.UserData.HP
}

func (user *dbUser) SetHP(hp uint64) {
	user.UserData.HP = hp
}

func (user *dbUser) MP() uint64 {
	return user.UserData.MP
}

func (user *dbUser) SetMP(mp uint64) {
	user.UserData.MP = mp
}

func (user *dbUser) AP() uint64 {
	return user.UserData.AP
}

func (user *dbUser) SetAP(ap uint64) {
	user.UserData.AP = ap
}

func (user *dbUser) MaxHP() uint64 {
	return user.UserData.MaxHP
}

func (user *dbUser) SetMaxHP(maxhp uint64) {
	user.UserData.MaxHP = maxhp
}

func (user *dbUser) MaxMP() uint64 {
	return user.UserData.MaxMP
}

func (user *dbUser) SetMaxMP(maxmp uint64) {
	user.UserData.MaxMP = maxmp
}

func (user *dbUser) MaxAP() uint64 {
	return user.UserData.MaxAP
}

func (user *dbUser) SetMaxAP(maxap uint64) {
	user.UserData.MaxAP = maxap
}

// Location returns the name of the current cell
func (user *dbUser) LocationName() string {
	ci := user.world.GetCellInfo(user.UserData.X, user.UserData.Y)
	if ci != nil {
		return ci.RegionName
	}
	return "Delaware"
}

func (user *dbUser) MoveNorth() {
	user.Reload()
	if user.Y > 0 {
		user.Y--
		user.Save()
	}
}

func (user *dbUser) MoveSouth() {
	user.Reload()
	_, height := user.world.GetDimensions()

	if user.Y < height-1 {
		user.Y++
		user.Save()
	}
}

func (user *dbUser) MoveEast() {
	user.Reload()
	width, _ := user.world.GetDimensions()

	if user.X < width-1 {
		user.X++
		user.Save()
	}
}

func (user *dbUser) MoveWest() {
	user.Reload()
	if user.X > 0 {
		user.X--
		user.Save()
	}
}

func (user *dbUser) Log(message string) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(buf, binary.BigEndian, byte(0))
	binary.Write(buf, binary.BigEndian, -time.Now().UnixNano())

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userlog"))

		err := bucket.Put(buf.Bytes(), []byte(message))

		return err
	})
}

func (user *dbUser) GetLog() []string {
	logMessages := make([]string, 0)

	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()
	ct := 0

	user.world.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userlog"))

		cur := bucket.Cursor()

		for k, v := cur.Seek(min); k != nil && bytes.Compare(k, max) <= 0 && ct < 80; k, v = cur.Next() {
			logMessages = append(logMessages, string(v))
			ct++
		}

		return nil
	})

	return logMessages
}

func (user *dbUser) MarkActive() {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, time.Now().UTC().Unix())

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("onlineusers"))
		err := bucket.Put([]byte(user.UserData.Username), buf.Bytes())

		return err
	})
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
	user := dbUser{UserData: UserData{
		Username: username},
		world: world}

	user.Reload()

	return &user
}
