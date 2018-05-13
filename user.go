package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"log"
	"time"

	bolt "github.com/coreos/bbolt"
)

// Strengths
const (
	MELEESECONDARY = byte(1)
	RANGESECONDARY = byte(2)
	MAGICSECONDARY = byte(3)
	MELEEPRIMARY   = byte(4)
	RANGEPRIMARY   = byte(8)
	MAGICPRIMARY   = byte(12)
)

// Skills
const (
	PEOPLESECONDARY = byte(16)
	PLACESSECONDARY = byte(32)
	THINGSSECONDARY = byte(48)
	PEOPLEPRIMARY   = byte(64)
	PLACESPRIMARY   = byte(128)
	THINGSPRIMARY   = byte(192)
)

// Masks for strenths/skills
const (
	SECONDARYSTRENGTHMASK = byte(3)
	PRIMARYSTRENGTHMASK   = byte(12)
	SECONDARYSKILLMASK    = byte(48)
	PRIMARYSKILLMASK      = byte(192)
)

// User represents an active user in the system.
type User interface {
	StatInfo
	ClassInfo
	LastAction
	ChargeInfo

	Username() string
	IsInitialized() bool
	Initialize(bool)
	Location() *Point

	MoveNorth()
	MoveSouth()
	MoveEast()
	MoveWest()
	ChargePoints()

	Log(message LogItem)
	GetLog() []LogItem

	MarkActive()
	LocationName() string

	Respawn()
	Reload()
	Save()
}

// StatInfo handles user/NPC stats
type StatInfo interface {
	HP() uint64
	SetHP(uint64)
	MP() uint64
	SetMP(uint64)
	AP() uint64
	SetAP(uint64)
	RP() uint64
	SetRP(uint64)
	MaxHP() uint64
	SetMaxHP(uint64)
	MaxMP() uint64
	SetMaxMP(uint64)
	MaxAP() uint64
	SetMaxAP(uint64)
	MaxRP() uint64
	SetMaxRP(uint64)
}

// ClassInfo handles user/NPC class orientation
type ClassInfo interface {
	ClassInfo() byte
	SetClassInfo(byte)

	Strengths() (byte, byte)
	SetStrengths(byte, byte)
	Skills() (byte, byte)
	SetSkills(byte, byte)
}

// LastAction tracks the last time an actor performed an action, for charging action bar.
type LastAction interface {
	Act()
	GetLastAction() int64
}

// ChargeInfo returns turn-base charge time info
type ChargeInfo interface {
	Charge() (int64, int64)
	Attacks() []*AttackInfo
	MusterAttack(string) *Attack
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
	RP          uint64          `json:""`
	MaxRP       uint64          `json:""`
	ClassInfo   byte            `json:""`
	Initialized bool            `json:""`
	PublicKeys  map[string]bool `json:""`
	Attacks     []*Attack       `json:""`
}

type dbUser struct {
	UserData
	world *dbWorld
}

// AttackInfo describes a user's attacks
type AttackInfo struct {
	Attack  *Attack
	Charged bool
}

func (user *dbUser) Username() string {
	return user.UserData.Username
}

func (user *dbUser) getDefaultAttacks() []*Attack {
	primary, secondary := user.Strengths()

	var primaryattack Attack
	var secondaryattack Attack

	switch primary {
	case MELEEPRIMARY:
		primaryattack = Attack{Name: "Punch",
			Accuracy: 95,
			MP:       0,
			AP:       4,
			RP:       0,
			Charge:   2}
	case RANGEPRIMARY:
		primaryattack = Attack{Name: "Dart",
			Accuracy: 95,
			MP:       0,
			AP:       0,
			RP:       4,
			Charge:   2}
	case MAGICPRIMARY:
		primaryattack = Attack{Name: "Mage push",
			Accuracy: 95,
			MP:       4,
			AP:       0,
			RP:       0,
			Charge:   2}
	}
	switch secondary {
	case MELEESECONDARY:
		secondaryattack = Attack{Name: "Biff",
			Accuracy: 95,
			MP:       0,
			AP:       2,
			RP:       0,
			Charge:   4}
		if primary == MELEEPRIMARY {
			secondaryattack.Charge = 1
		} else if primary == RANGEPRIMARY {
			secondaryattack.RP++
		} else if primary == MAGICPRIMARY {
			secondaryattack.MP++
		}
	case RANGESECONDARY:
		secondaryattack = Attack{Name: "Toss",
			Accuracy: 95,
			MP:       0,
			AP:       0,
			RP:       2,
			Charge:   4}
		if primary == RANGEPRIMARY {
			secondaryattack.Charge = 1
		} else if primary == MELEEPRIMARY {
			secondaryattack.AP++
		} else if primary == MAGICPRIMARY {
			secondaryattack.MP++
		}
	case MAGICSECONDARY:
		secondaryattack = Attack{Name: "Crackle",
			Accuracy: 95,
			MP:       2,
			AP:       0,
			RP:       0,
			Charge:   4}
		if primary == MAGICPRIMARY {
			secondaryattack.Charge = 1
		} else if primary == RANGEPRIMARY {
			secondaryattack.RP++
		} else if primary == MELEEPRIMARY {
			secondaryattack.AP++
		}
	}

	attacks := []*Attack{&primaryattack, &secondaryattack}

	return attacks
}

func (user *dbUser) setupStatBonuses() {
	primary, secondary := user.Strengths()

	switch primary {
	case MELEEPRIMARY:
		user.UserData.MaxAP *= 3
	case RANGEPRIMARY:
		user.UserData.MaxRP *= 3
	case MAGICPRIMARY:
		user.UserData.MaxMP *= 3
	}
	switch secondary {
	case MELEESECONDARY:
		user.UserData.MaxAP *= 2
	case RANGESECONDARY:
		user.UserData.MaxRP *= 2
	case MAGICSECONDARY:
		user.UserData.MaxMP *= 2
	}
}

func (user *dbUser) Initialize(initialize bool) {
	user.Reload()

	user.UserData.Attacks = user.getDefaultAttacks()
	user.setupStatBonuses()
	user.Initialized = initialize
	user.Save()
}

func (user *dbUser) IsInitialized() bool {
	return user.UserData.Initialized
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

func (user *dbUser) RP() uint64 {
	return user.UserData.RP
}

func (user *dbUser) SetRP(rp uint64) {
	user.UserData.RP = rp
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

func (user *dbUser) MaxRP() uint64 {
	return user.UserData.MaxRP
}

func (user *dbUser) SetMaxRP(maxrp uint64) {
	user.UserData.MaxRP = maxrp
}

func (user *dbUser) ClassInfo() byte {
	return user.UserData.ClassInfo
}

func (user *dbUser) SetClassInfo(classinfo byte) {
	user.Reload()
	user.UserData.ClassInfo = classinfo
	user.Save()
}

func (user *dbUser) Strengths() (byte, byte) {
	return user.UserData.ClassInfo & PRIMARYSTRENGTHMASK, user.UserData.ClassInfo & SECONDARYSTRENGTHMASK
}

func (user *dbUser) SetStrengths(primary, secondary byte) {
	primary &= PRIMARYSTRENGTHMASK
	secondary &= SECONDARYSTRENGTHMASK

	user.Reload()
	user.UserData.ClassInfo = (user.UserData.ClassInfo &^ (PRIMARYSTRENGTHMASK | SECONDARYSTRENGTHMASK)) | primary | secondary
	user.Save()
}

func (user *dbUser) Skills() (byte, byte) {
	return user.UserData.ClassInfo & PRIMARYSKILLMASK, user.UserData.ClassInfo & SECONDARYSKILLMASK
}

func (user *dbUser) SetSkills(primary, secondary byte) {
	primary &= PRIMARYSKILLMASK
	secondary &= SECONDARYSKILLMASK

	user.Reload()
	user.UserData.ClassInfo = (user.UserData.ClassInfo &^ (PRIMARYSKILLMASK | SECONDARYSKILLMASK)) | primary | secondary
	user.Save()
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
		user.world.activateCell(user.X, user.Y)
		user.Act()
		user.Save()
	}
}

func (user *dbUser) MoveSouth() {
	user.Reload()
	_, height := user.world.GetDimensions()

	if user.Y < height-1 {
		user.Y++
		user.world.activateCell(user.X, user.Y)
		user.Act()
		user.Save()
	}
}

func (user *dbUser) MoveEast() {
	user.Reload()
	width, _ := user.world.GetDimensions()

	if user.X < width-1 {
		user.X++
		user.world.activateCell(user.X, user.Y)
		user.Act()
		user.Save()
	}
}

func (user *dbUser) MoveWest() {
	user.Reload()
	if user.X > 0 {
		user.X--
		user.world.activateCell(user.X, user.Y)
		user.Act()
		user.Save()
	}
}

func (user *dbUser) ChargePoints() {
	user.Reload()
	ap := user.MaxAP()
	rp := user.MaxRP()
	mp := user.MaxMP()
	cap := user.AP()
	crp := user.RP()
	cmp := user.MP()

	full := true

	if cap < ap {
		user.SetAP(cap + 1)
		full = false
	}

	if crp < rp {
		user.SetRP(crp + 1)
		full = false
	}

	if cmp < mp {
		user.SetMP(cmp + 1)
		full = false
	}

	if full {
		hp, maxhp := user.HP(), user.MaxHP()

		if hp == 0 {
			user.Respawn()
		} else {
			chg, maxchg := user.Charge()
			if hp < maxhp && chg == maxchg && time.Now().Unix()%5 == 0 {
				user.SetHP(hp + 1)
			}
		}
	}

	user.Save()
}

func (user *dbUser) Log(message LogItem) {
	now := time.Now().UTC()

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(buf, binary.BigEndian, byte(0))
	binary.Write(buf, binary.BigEndian, -now.UnixNano())

	messageBytes, err := json.Marshal(message)

	if err != nil {
		log.Printf("Log serialization failure: %v", err)
		return
	}

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userlog"))

		err := bucket.Put(buf.Bytes(), messageBytes)

		return err
	})
}

func (user *dbUser) GetLog() []LogItem {
	logMessages := make([]LogItem, 0)

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
			var messageStruct LogItem

			err := json.Unmarshal(v, &messageStruct)

			if err != nil {
				return err
			}

			logMessages = append(logMessages, messageStruct)
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

	user.world.activateCell(user.X, user.Y)
}

func (user *dbUser) Respawn() {
	user.X = user.SpawnX
	user.Y = user.SpawnY
	user.SetHP(user.MaxHP())
	user.Log(LogItem{Message: "You died. Be more careful.", MessageType: MESSAGESYSTEM})
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

func (user *dbUser) Act() {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, time.Now().UTC().UnixNano())

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("lastuseraction"))
		return bucket.Put([]byte(user.UserData.Username), buf.Bytes())
	})
}

func (user *dbUser) GetLastAction() int64 {
	timeDelta := int64(0)

	user.world.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("lastuseraction"))
		stamp := bucket.Get([]byte(user.UserData.Username))
		buf := bytes.NewBuffer(stamp)

		var last int64

		if binary.Read(buf, binary.BigEndian, &last) == nil {
			timeDelta = time.Now().UTC().UnixNano() - last
		}

		return nil
	})

	return timeDelta / 1000000000
}

func (user *dbUser) Charge() (int64, int64) {
	charge := user.GetLastAction()
	maxCharge := int64(user.MaxAP()+user.MaxMP()+user.MaxRP()) / 3
	if charge > maxCharge {
		charge = maxCharge
	}

	return charge, maxCharge
}

func (user *dbUser) Attacks() []*AttackInfo {
	attacks := make([]*AttackInfo, 0)

	charge, _ := user.Charge()

	for _, item := range user.UserData.Attacks {
		attack := *item
		charged := charge >= attack.Charge && user.AP() >= attack.AP && user.RP() >= attack.RP && user.MP() >= attack.MP

		attacks = append(attacks, &AttackInfo{Attack: &attack, Charged: charged})
	}

	return attacks
}

func (user *dbUser) MusterAttack(attackName string) *Attack {
	for _, attack := range user.Attacks() {
		if attack.Attack.Name == attackName {
			potentialAttack := attack.Attack

			user.Reload()
			charge, _ := user.Charge()

			if charge >= potentialAttack.Charge {

				ap, rp, mp := user.AP(), user.RP(), user.MP()
				if ap >= potentialAttack.AP && rp >= potentialAttack.RP && mp >= potentialAttack.MP {
					user.SetAP(ap - potentialAttack.AP)
					user.SetRP(rp - potentialAttack.RP)
					user.SetMP(mp - potentialAttack.MP)
					user.Save()
					user.Act()

					return &*potentialAttack
				}
			}
		}
	}

	return nil
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
