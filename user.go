package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	bolt "github.com/coreos/bbolt"
)

// EquipUserInfo is for putting outfits on a user
type EquipUserInfo interface {
	Equip(*InventoryItem) (*InventoryItem, error)
	Equipped() []*InventoryItem
}

// User represents an active user in the system.
type User interface {
	StatInfo
	ClassInfo
	LastAction
	ChargeInfo
	InventoryInfo
	EquipUserInfo

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
	MusterCounterAttack() *Attack
}

// UserSSHAuthentication for storing SSH auth.
type UserSSHAuthentication interface {
	SSHKeysEmpty() bool
	ValidateSSHKey(string) bool
	AddSSHKey(string)
}

// UserData is a JSON-serializable set of information about a User.
type UserData struct {
	Username    string           `json:""`
	X           uint32           `json:""`
	Y           uint32           `json:""`
	SpawnX      uint32           `json:""`
	SpawnY      uint32           `json:""`
	HP          uint64           `json:""`
	MaxHP       uint64           `json:""`
	AP          uint64           `json:""`
	MaxAP       uint64           `json:""`
	MP          uint64           `json:""`
	MaxMP       uint64           `json:""`
	RP          uint64           `json:""`
	MaxRP       uint64           `json:""`
	XP          uint64           `json:""`
	ClassInfo   byte             `json:""`
	Initialized bool             `json:""`
	PublicKeys  map[string]bool  `json:""`
	Equipped    []*InventoryItem `json:""`
	Attacks     []*Attack        `json:""`
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
	rockAttack := Attack{
		Name:         "Rock user",
		Accuracy:     100,
		MP:           1,
		AP:           1,
		RP:           1,
		Trample:      6,
		Charge:       1,
		UsesItems:    []string{"Shiny Rock"},
		OutputsItems: []string{"Broken Rock"}}

	switch primary {
	case MELEEPRIMARY:
		primaryattack = Attack{Name: "Punch",
			Accuracy: 95,
			MP:       0,
			AP:       4,
			RP:       0,
			Charge:   2}

		rockAttack.Name = "Smash Rock"
		rockAttack.AP *= 2
		rockAttack.Bonuses = "AP+25%AP"
	case RANGEPRIMARY:
		primaryattack = Attack{Name: "Dart",
			Accuracy: 95,
			MP:       0,
			AP:       0,
			RP:       4,
			Charge:   2}

		rockAttack.Name = "Throw Rock"
		rockAttack.RP *= 2
		rockAttack.Bonuses = "RP+25%RP"
	case MAGICPRIMARY:
		primaryattack = Attack{Name: "Mage push",
			Accuracy: 95,
			MP:       4,
			AP:       0,
			RP:       0,
			Charge:   2}

		rockAttack.Name = "Rock Bomb"
		rockAttack.MP *= 2
		rockAttack.Bonuses = "MP+25%MP"
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
			secondaryattack.Trample = 1
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
			secondaryattack.Trample = 1
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
			secondaryattack.Trample = 1
		} else if primary == RANGEPRIMARY {
			secondaryattack.RP++
		} else if primary == MELEEPRIMARY {
			secondaryattack.AP++
		}
	}

	attacks := []*Attack{&primaryattack, &secondaryattack, &rockAttack}

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
	user.UserData.Equipped = make([]*InventoryItem, 0)
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

func (user *dbUser) XP() uint64 {
	return user.UserData.XP
}

func (user *dbUser) AddXP(xp uint64) {
	user.Reload()
	user.UserData.XP += xp
	user.Save()
}

func (user *dbUser) XPToNextLevel() uint64 {
	return user.MaxAP() + user.MaxRP() + user.MaxMP()
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
	ap, rp, mp, xp := user.MaxAP(), user.MaxRP(), user.MaxMP(), user.XPToNextLevel()
	cap, crp, cmp, cxp := user.AP(), user.RP(), user.MP(), user.XP()

	changed := false
	full := true

	if cap < ap {
		user.SetAP(cap + 1)
		changed = true
		full = false
	}

	if crp < rp {
		user.SetRP(crp + 1)
		changed = true
		full = false
	}

	if cmp < mp {
		user.SetMP(cmp + 1)
		changed = true
		full = false
	}

	if cxp >= xp {
		user.levelUp()
		changed = true
	}

	if full {
		hp, maxhp := user.HP(), user.MaxHP()

		if hp == 0 {
			user.Save()
			user.Respawn()
		} else {
			chg, maxchg := user.Charge()
			if hp < maxhp && chg == maxchg && time.Now().Unix()%5 == 0 {
				user.SetHP(hp + 1)
				changed = true
			}
		}
	}

	if changed {
		user.Save()
	}
}

func (user *dbUser) levelUp() {
	if user.XP() >= user.XPToNextLevel() {
		var apbonus, rpbonus, mpbonus, hpbonus uint64 = 1, 1, 1, 1

		user.UserData.XP -= user.XPToNextLevel()

		primary, secondary := user.Strengths()

		switch primary {
		case MELEEPRIMARY:
			apbonus += 2
		case RANGEPRIMARY:
			rpbonus += 2
		case MAGICPRIMARY:
			mpbonus += 2
		}

		switch secondary {
		case MELEESECONDARY:
			apbonus++
		case RANGESECONDARY:
			rpbonus++
		case MAGICSECONDARY:
			mpbonus++
		}

		user.SetMaxAP(user.MaxAP() + apbonus)
		user.SetMaxRP(user.MaxRP() + rpbonus)
		user.SetMaxMP(user.MaxMP() + mpbonus)
		user.SetMaxHP(user.MaxHP() + hpbonus)

		user.Log(LogItem{Message: "Leveled Up!", MessageType: MESSAGEACTIVITY})
	}
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
	user.Reload()
	if user.HP() == 0 {
		user.SetHP(user.MaxHP())
		user.Save()
		user.Log(LogItem{Message: "You died. Be more careful.", MessageType: MESSAGESYSTEM})
		user.X = user.SpawnX
		user.Y = user.SpawnY
		user.SetHP(user.MaxHP())
		user.Save()
	}
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

	for _, inventoryItem := range user.UserData.Equipped {
		if inventoryItem.Type == ITEMTYPEWEAPON {
			for _, attack := range inventoryItem.Attacks {
				atk := attack
				charged := charge >= attack.Charge && user.AP() >= attack.AP && user.RP() >= attack.RP && user.MP() >= attack.MP
				attacks = append(attacks, &AttackInfo{Attack: &atk, Charged: charged})
			}
		}
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
				hasItems := true
				missingItem := "true and pure soul"
				if attack.Attack.UsesItems != nil {
					itemsToTake := make([]*InventoryItem, len(attack.Attack.UsesItems))
				ItemIter:
					for _, item := range attack.Attack.UsesItems {
						itemAttack := user.pullInventoryItemByName(item)
						if itemAttack == nil {
							hasItems = false
							missingItem = item
							break ItemIter
						}
					}

					if !hasItems {
						user.Log(LogItem{Message: fmt.Sprintf("You lack %v required to %v", missingItem, potentialAttack.Name), MessageType: MESSAGEACTIVITY})
						for _, item := range itemsToTake {
							if !user.AddInventoryItem(item) {
								user.world.AddInventoryItem(user.X, user.Y, item)
							}
						}

						return nil
					}
					if attack.Attack.OutputsItems != nil {
						for _, itemName := range attack.Attack.OutputsItems {
							item, ok := ItemTypes[itemName]
							if ok {
								user.world.AddInventoryItem(user.X, user.Y, &item)
							}
						}
					}
				}

				ap, rp, mp := user.AP(), user.RP(), user.MP()
				if ap >= potentialAttack.AP && rp >= potentialAttack.RP && mp >= potentialAttack.MP {
					user.SetAP(ap - potentialAttack.AP)
					user.SetRP(rp - potentialAttack.RP)
					user.SetMP(mp - potentialAttack.MP)
					user.Save()
					user.Act()

					userSP := GetStatPoints(user)
					attack := potentialAttack.ApplyBonuses(&userSP)

					return &attack
				}
			}
		}
	}

	return nil
}

func (user *dbUser) MusterCounterAttack() *Attack {
	return nil
}

func (user *dbUser) InventoryItems() []*InventoryItem {
	items := make([]*InventoryItem, 0)

	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()

	user.world.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userinventory"))

		cur := bucket.Cursor()

		for k, v := cur.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = cur.Next() {
			var inventoryItem InventoryItem

			err := json.Unmarshal(v, &inventoryItem)

			if err != nil {
				return err
			}

			items = append(items, &inventoryItem)
		}

		return nil
	})

	return items
}

func (user *dbUser) AddInventoryItem(item *InventoryItem) bool {
	if item == nil {
		return false
	}

	inventoryItem := *item

	if inventoryItem.ID == "" {
		inventoryItem.ID = uuid.New().String()
	}

	itemID, err := uuid.Parse(inventoryItem.ID)
	if err != nil {
		return false
	}

	idBytes, err := itemID.MarshalBinary()
	if err != nil {
		return false
	}

	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)
	dataBytes, err := json.Marshal(inventoryItem)

	if err != nil {
		return false
	}

	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userinventory"))

		return bucket.Put(keyBuf.Bytes(), dataBytes)
	})

	return true
}

func (user *dbUser) inventoryItem(id string, pull bool) *InventoryItem {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return nil
	}

	idBytes, err := itemID.MarshalBinary()
	if err != nil {
		return nil
	}

	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, []byte(user.UserData.Username))
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)

	found := false
	var inventoryItem InventoryItem
	user.world.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("userinventory"))

		itemBytes := bucket.Get(keyBuf.Bytes())

		if itemBytes != nil {
			if json.Unmarshal(itemBytes, &inventoryItem) == nil {
				inventoryItem.ID = id
				found = true
			}
		}

		if pull && found {
			bucket.Delete(keyBuf.Bytes())
		}

		return nil
	})

	if found {
		return &inventoryItem
	}
	return nil
}

func (user *dbUser) InventoryItem(id string) *InventoryItem {
	return user.inventoryItem(id, false)
}

func (user *dbUser) PullInventoryItem(id string) *InventoryItem {
	return user.inventoryItem(id, true)
}

func (user *dbUser) pullInventoryItemByName(name string) *InventoryItem {
	for _, item := range user.InventoryItems() {
		if item.Name == name {
			return user.PullInventoryItem(item.ID)
		}
	}

	return nil
}

func (user *dbUser) Equip(item *InventoryItem) (*InventoryItem, error) {
	var toss *InventoryItem
	var err error

	index := -1

InvIter:
	for idx, inventoryItem := range user.UserData.Equipped {
		if inventoryItem.Type == item.Type {
			index = idx
			break InvIter
		}
	}

	if index != -1 {
		toss = user.UserData.Equipped[index]
		user.UserData.Equipped = append(user.UserData.Equipped[:index], user.UserData.Equipped[index+1:]...)
	}

	if item.Type == ITEMTYPEWEAPON {
		user.UserData.Equipped = append(user.UserData.Equipped, item)
	} else {
		err = fmt.Errorf("Can't equip %v", item.Name)
		if toss != nil {
			user.UserData.Equipped = append(user.UserData.Equipped, toss)
		}
		toss = item
	}

	return toss, err
}

func (user *dbUser) Equipped() []*InventoryItem {
	inventory := make([]*InventoryItem, 0)
	for _, i := range user.UserData.Equipped {
		ic := *i
		inventory = append(inventory, &ic)
	}

	return inventory
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
