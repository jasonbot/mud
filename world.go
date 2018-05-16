package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"

	bolt "github.com/coreos/bbolt"
)

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
	GetUser(string) User

	GetCellInfo(uint32, uint32) *CellInfo
	SetCellInfo(uint32, uint32, *CellInfo)

	GetCreatures(uint32, uint32) []*Creature
	HasCreatures(uint32, uint32) bool
	UpdateCreature(*Creature)
	ClearCreatures(uint32, uint32)
	AddStockCreature(uint32, uint32, string)
	KillCreature(string)
	Attack(interface{}, interface{}, *Attack)

	InventoryItems(uint32, uint32) []*InventoryItem
	AddInventoryItem(uint32, uint32, *InventoryItem) bool
	InventoryItem(uint32, uint32, string) *InventoryItem
	PullInventoryItem(uint32, uint32, string) *InventoryItem
	HasInventoryItems(uint32, uint32) bool

	NewPlaceID() uint64
	OnlineUsers() []User
	Chat(LogItem)
	Close()
}

type dbWorld struct {
	filename         string
	database         *bolt.DB
	closeActiveCells chan struct{}
	activeCellCache  sync.Map
}

type recentCellInfo struct {
	x, y                  uint32
	lastVisit             int64
	cellInfo              *CellInfo
	creatures             []*Creature
	lastCreatureAction    map[string]int64
	desiredCreatureCharge map[string]int64
}

func (w *dbWorld) chargeUsers() {
	for _, user := range w.OnlineUsers() {
		user.ChargePoints()
	}
}

const cachedCellExpirationAge = 10

func (recent *recentCellInfo) IsExpired() bool {
	now := time.Now().Unix()
	if (now - recent.lastVisit) > cachedCellExpirationAge {
		return true
	}
	return false
}

func (w *dbWorld) activateCell(x, y uint32) {
	pt := Point{x, y}
	key := string(pt.Bytes())
	now := time.Now().Unix()

	rci := &recentCellInfo{
		x:                     x,
		y:                     y,
		lastVisit:             now,
		cellInfo:              w.GetCellInfo(x, y),
		creatures:             w.getCreatures(x, y),
		lastCreatureAction:    make(map[string]int64),
		desiredCreatureCharge: make(map[string]int64)}

	ci, _ := w.activeCellCache.LoadOrStore(key, rci)
	cell, ok := ci.(*recentCellInfo)

	if ok {
		cell.lastVisit = now
	}
}

func (w *dbWorld) updateActivatedCells() {
	now := time.Now().Unix()

	w.activeCellCache.Range(func(k, v interface{}) bool {
		cell, ok := v.(*recentCellInfo)

		if ok {
			for _, creature := range cell.creatures {
				if creature.HP <= 0 {
					continue
				}

				lastAction, ok := cell.lastCreatureAction[creature.ID]

				if !ok {
					cell.lastCreatureAction[creature.ID] = now
					lastAction = now
				}

				creature.Charge = now - lastAction
				if creature.Charge > creature.maxCharge {
					creature.Charge = creature.maxCharge
				}

				desiredLevel, ok := cell.desiredCreatureCharge[creature.ID]
				resetLevel := false

				if !ok || desiredLevel == 0 {
					resetLevel = true
				} else if desiredLevel <= creature.Charge {
					location := Point{X: creature.X, Y: creature.Y}
					resetLevel = true
					attack := creature.CreatureTypeStruct.Attacks[rand.Int()%len(creature.CreatureTypeStruct.Attacks)]
					csp := creature.StatPoints()
					attack = attack.ApplyBonuses(&csp)
					if attack.Charge <= creature.Charge {
						usersInCell := w.usersInCell(location)

						if len(usersInCell) > 0 {
							user := usersInCell[rand.Int()%len(usersInCell)]
							user.Reload()
							if *(user.Location()) == location {
								w.Attack(creature, user, &attack)
								cell.lastCreatureAction[creature.ID] = now
							}
						}
					}
				}

				if resetLevel {
					if (creature.maxCharge) > 0 {
						desiredLevel = int64(1 + rand.Int()%int(creature.maxCharge))
						cell.desiredCreatureCharge[creature.ID] = desiredLevel
					}
				}
			}
		}

		return true
	})
}

func (w *dbWorld) sweepExpiredKeys() {
	keys := make([]string, 0)

	w.activeCellCache.Range(func(k, v interface{}) bool {
		key, ok := k.(string)
		if !ok {
			return false
		}

		value, ok := v.(*recentCellInfo)
		if !ok {
			return false
		}

		if value.IsExpired() {
			keys = append(keys, key)
		}

		return true
	})
	for _, key := range keys {
		v, _ := w.activeCellCache.Load(key)
		value, ok := v.(*recentCellInfo)
		if ok {
			for _, creature := range value.creatures {
				if creature.HP <= 0 {
					w.KillCreature(creature.ID)
				}
			}
		}
		w.activeCellCache.Delete(key)
	}
}

// GetDimensions returns the size of the world
func (w *dbWorld) GetDimensions() (uint32, uint32) {
	return uint32(1 << 30), uint32(1 << 30)
}

func (w *dbWorld) GetUser(username string) User {
	return getUserFromDB(w, username)
}

func (w *dbWorld) newUser(username string) UserData {
	width, height := w.GetDimensions()
	userData := UserData{
		Username:   username,
		X:          width / 2,
		Y:          height / 2,
		SpawnX:     width / 2,
		SpawnY:     height / 2,
		HP:         10,
		MaxHP:      10,
		AP:         2,
		MaxAP:      2,
		MP:         2,
		MaxMP:      2,
		RP:         2,
		MaxRP:      2,
		PublicKeys: make(map[string]bool)}
	cellData := w.GetCellInfo(userData.X, userData.Y)

	if cellData == nil {
		newRegionID, _ := newPlaceNameInDB(w.database)
		cellData = &CellInfo{
			TerrainID:    DefaultCellType,
			RegionNameID: newRegionID}

		w.SetCellInfo(userData.X, userData.Y, cellData)
	}

	return userData
}

func (w *dbWorld) GetCellInfo(x, y uint32) *CellInfo {
	var cellInfo CellInfo
	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{x, y}
		record := bucket.Get(pt.Bytes())

		if record != nil {
			cellInfo = cellInfoFromBytes(record)
		}

		return nil
	})

	placeName := getPlaceNameByIDFromDB(cellInfo.RegionNameID, w.database)
	cellTerrain, ok := CellTypes[cellInfo.TerrainID]

	if ok {
		// Format place name if it exists
		if len(cellTerrain.Name) > 0 {
			placeName = fmt.Sprintf(cellTerrain.Name, placeName)
		}

		cellInfo.RegionName = placeName
		cellInfo.TerrainData = cellTerrain

		return &cellInfo
	}

	return nil
}

func (w *dbWorld) SetCellInfo(x, y uint32, cellInfo *CellInfo) {
	pt := Point{x, y}
	key := pt.Bytes()

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		bytes := cellInfoToBytes(cellInfo)
		err := bucket.Put(key, bytes)

		return err
	})

	_, ok := w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		w.activeCellCache.Store(string(key), &recentCellInfo{x: x, y: y, lastVisit: time.Now().Unix(), cellInfo: cellInfo})
	}

	ct, ok := CellTypes[cellInfo.TerrainID]

	var spawns []CreatureSpawn
	var drops []ItemDrop

	if ok {
		spawns = ct.CreatureSpawns
		drops = ct.ItemDrops
	}

	if spawns != nil {
		for _, spawn := range spawns {
			cl := spawn.Cluster
			if cl < 1 {
				cl = 1
			}

			prob := rand.Float32()
			for clusterCount := 0; clusterCount < int(cl); clusterCount++ {

				if spawn.Probability >= prob {
					if clusterCount > 0 {
						prob += (spawn.Probability / 2.0)
					}

					w.AddStockCreature(x, y, spawn.Name)
				}
			}
		}
	}

	if drops != nil {
		for _, drop := range drops {
			cluster := drop.Cluster
			if cluster == 0 {
				cluster = 1
			}

			for i := 0; i < int(cluster); i++ {
				prob := rand.Float32()
				if drop.Probability >= prob {
					dropItem := ItemTypes[drop.Name]
					w.AddInventoryItem(x, y, &dropItem)
				}
			}
		}
	}
}

func (w *dbWorld) reloadStoredCreatures(x, y uint32) {
	pt := Point{X: x, Y: y}
	record, ok := w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast {
			ci.creatures = w.getCreatures(x, y)
		}
	}
}

func (w *dbWorld) creatureList(x, y uint32) []string {
	var cl CreatureList

	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("creaturelist"))

		pt := Point{X: x, Y: y}

		bytes := bucket.Get(pt.Bytes())

		if bytes != nil {
			json.Unmarshal(bytes, &cl)
		}

		return nil
	})

	return cl.CreatureIDs
}

func (w *dbWorld) getCreature(id string) *Creature {
	var creature *Creature

	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("creatures"))

		id, err := uuid.Parse(id)

		if err != nil {
			return err
		}

		byteID, err := id.MarshalBinary()

		if err != nil {
			return err
		}

		recordBytes := bucket.Get(byteID)

		if recordBytes != nil {
			creature = &Creature{}
			json.Unmarshal(recordBytes, creature)
		}

		return err
	})

	creature.CreatureTypeStruct = CreatureTypes[creature.CreatureType]
	creature.maxCharge = int64(creature.CreatureTypeStruct.MaxAP+creature.CreatureTypeStruct.MaxMP+creature.CreatureTypeStruct.MaxRP) / 3
	creature.Charge = 0

	return creature
}

func (w *dbWorld) getCreatures(x, y uint32) []*Creature {
	cl := w.creatureList(x, y)
	creatures := make([]*Creature, 0)

	nameFixers := make(map[string]int)

	if cl != nil && len(cl) > 0 {
		for _, id := range cl {
			c := w.getCreature(id)
			if c != nil {
				_, gotName := nameFixers[c.CreatureTypeStruct.Name]
				if gotName {
					nameFixers[c.CreatureTypeStruct.Name] = nameFixers[c.CreatureTypeStruct.Name] + 1
					c.CreatureTypeStruct.Name = fmt.Sprintf("%v (%v)", c.CreatureTypeStruct.Name, nameFixers[c.CreatureTypeStruct.Name])
				} else {
					nameFixers[c.CreatureTypeStruct.Name] = 1
				}

				for _, attack := range c.CreatureTypeStruct.Attacks {
					if attack.Charge > c.maxCharge {
						c.maxCharge = attack.Charge
					}
				}

				creatures = append(creatures, c)
			}
		}
	}

	return creatures
}

func (w *dbWorld) GetCreatures(x, y uint32) []*Creature {
	pt := Point{X: x, Y: y}
	record, ok := w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast && ci.creatures != nil {
			return ci.creatures
		}
	}

	return w.getCreatures(x, y)
}

func (w *dbWorld) HasCreatures(x, y uint32) bool {
	pt := Point{X: x, Y: y}
	record, ok := w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast && ci.creatures != nil {
			if len(ci.creatures) > 0 {
				return true
			}

			return false
		}
	}

	cl := w.creatureList(x, y)

	if cl == nil || len(cl) == 0 {
		return false
	}

	for _, id := range cl {
		c := w.getCreature(id)
		if c != nil {
			return true
		}
	}

	return false
}

func (w *dbWorld) UpdateCreature(creature *Creature) {
	cID, err := uuid.Parse(creature.ID)

	if err != nil {
		return
	}

	w.database.Update(func(tx *bolt.Tx) error {
		creatureBucket := tx.Bucket([]byte("creatures"))

		creatureBytes, err := json.Marshal(creature)

		if err != nil {
			return err
		}

		byteKey, err := cID.MarshalBinary()

		if err != nil {
			return err
		}

		return creatureBucket.Put(byteKey, creatureBytes)
	})

	w.reloadStoredCreatures(creature.X, creature.Y)
}

func (w *dbWorld) ClearCreatures(x, y uint32) {
	creatures := w.creatureList(x, y)

	if creatures != nil {
		for _, id := range creatures {
			w.KillCreature(id)
		}
	}

	w.reloadStoredCreatures(x, y)
}

func (w *dbWorld) AddStockCreature(x, y uint32, id string) {
	cID := uuid.New()
	creatureType := CreatureTypes[id]
	creature := &Creature{
		ID:           cID.String(),
		CreatureType: creatureType.ID,
		X:            x,
		Y:            y,
		HP:           creatureType.MaxHP,
		AP:           creatureType.MaxAP,
		MP:           creatureType.MaxMP,
		RP:           creatureType.MaxRP,
		world:        w}

	creatureList := CreatureList{}

	w.database.Update(func(tx *bolt.Tx) error {
		creatureBucket := tx.Bucket([]byte("creatures"))
		creatureListBucket := tx.Bucket([]byte("creaturelist"))

		creatureBytes, err := json.Marshal(creature)

		if err != nil {
			return err
		}

		byteKey, err := cID.MarshalBinary()

		if err != nil {
			return err
		}

		err = creatureBucket.Put(byteKey, creatureBytes)

		if err != nil {
			return err
		}

		pt := Point{X: x, Y: y}
		creatureListBytes := creatureListBucket.Get(pt.Bytes())

		if creatureListBytes != nil {
			err = json.Unmarshal(creatureListBytes, &creatureList)

			if err != nil {
				return err
			}
		}

		if creatureList.CreatureIDs == nil {
			creatureList.CreatureIDs = make([]string, 0)
		}

		creatureList.CreatureIDs = append(creatureList.CreatureIDs, creature.ID)
		creatureListBytes, _ = json.Marshal(creatureList)
		creatureListBucket.Put(pt.Bytes(), creatureListBytes)

		return nil
	})
}

func (w *dbWorld) KillCreature(id string) {
	cID, err := uuid.Parse(id)

	if err != nil {
		return
	}

	byteID, err := cID.MarshalBinary()

	if err != nil {
		return
	}

	creature := w.getCreature(id)

	if creature == nil {
		return
	}

	location := Point{X: creature.X, Y: creature.Y}

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("creatures"))
		err := bucket.Delete(byteID)

		if err != nil {
			return err
		}

		creatureListBucket := tx.Bucket([]byte("creaturelist"))
		creatureListBytes := creatureListBucket.Get(location.Bytes())

		if creatureListBytes != nil {
			var creatureList CreatureList
			err = json.Unmarshal(creatureListBytes, &creatureList)

			if err != nil {
				return err
			}

			aliveCreatureList := make([]string, 0)

			if creatureList.CreatureIDs != nil {
				for _, cid := range creatureList.CreatureIDs {
					cuid, err := uuid.Parse(cid)

					if err != nil {
						return err
					}

					idBytes, err := cuid.MarshalBinary()

					if err != nil {
						return err
					}

					b := bucket.Get(idBytes)

					if b != nil {
						aliveCreatureList = append(aliveCreatureList, cid)
					}
				}
			}

			creatureList.CreatureIDs = aliveCreatureList
			creatureListBytes, _ = json.Marshal(creatureList)
			creatureListBucket.Put(location.Bytes(), creatureListBytes)
		}

		return nil
	})
}

func (w *dbWorld) Attack(source interface{}, target interface{}, attack *Attack) {
	var counterAttack *Attack

	if attack == nil {
		log.Println("Attack is nil")
		return
	}
	var targetpoints StatPoints

	message := ""
	hitTarget := "target"
	sourceString := "Something"

	sourceUser, sourceUserok := source.(User)
	sourceCreature, sourceCreatureok := source.(*Creature)

	if sourceUserok {
		sourceString = sourceUser.Username()
	} else if sourceCreatureok {
		sourceString = sourceCreature.CreatureTypeStruct.Name
	}

	user, userok := target.(User)
	creature, creatureok := target.(*Creature)

	var location *Point

	if userok {
		user.Reload()
		targetpoints = GetStatPoints(user)
		location = user.Location()
		hitTarget = user.Username()
	} else if creatureok {
		targetpoints = creature.StatPoints()
		location = &Point{X: creature.X, Y: creature.Y}
		hitTarget = creature.CreatureTypeStruct.Name
	}

	hit := rand.Int()%100 < int(attack.Accuracy)
	killed := false

	if hit {
		attackpoints := attack.StatPoints()
		damagepoints := attackpoints.ApplyDefense(&targetpoints)
		damage := damagepoints.Damage()
		if attack.Trample > 0 {
			damage += uint64(rand.Int() % int(attack.Trample+1))
		}

		if userok {
			user.Reload()

			if damage > 0 {
				counterAttack = user.MusterCounterAttack()
			}

			if user.HP() == 0 {
				message = fmt.Sprintf("%v is already dead, attack failed.", user.Username())
			}

			if counterAttack != nil {
				if user.HP() > damage {
					user.SetHP(user.HP() - damage)
				} else {
					user.SetHP(0)
					killed = true
				}
			}

			user.Save()
		} else if creatureok {
			if creature.HP > damage {
				creature.HP -= damage
			} else {
				creature.HP = 0
				w.creatureDrop(creature)
				killed = true
			}

			w.UpdateCreature(creature)

			if killed {
				for _, user := range w.usersInCell(Point{X: creature.X, Y: creature.Y}) {
					user.AddXP(uint64(creature.maxCharge))
				}
			}
		} else {
			log.Printf("How do I handle %v for attacks?", target)
		}

		if killed {
			message = fmt.Sprintf("%v took fatal damage from %v!", hitTarget, attack.Name)
		} else if len(message) == 0 {
			if counterAttack == nil {
				message = fmt.Sprintf("%v hit %v for %v damage!", attack.Name, hitTarget, damage)
			} else {
				message = fmt.Sprintf("Attempted %v against %v; blocked with a counterattack!", attack.Name, hitTarget)
			}
		}
	} else {
		message = fmt.Sprintf("%v missed!", attack.Name)
	}

	if len(message) > 0 {
		w.Chat(LogItem{Author: sourceString, Message: message, MessageType: MESSAGEACTIVITY, Location: location})
	}

	if counterAttack != nil {
		w.Attack(target, source, counterAttack)
	}
}

func (w *dbWorld) creatureDrop(creature *Creature) {
	drops := creature.CreatureTypeStruct.ItemDrops

	if drops != nil && len(drops) > 0 {
		for _, drop := range drops {
			cluster := drop.Cluster
			if cluster == 0 {
				cluster = 1
			}

			for i := 0; i < int(cluster); i++ {
				prob := rand.Float32()
				if drop.Probability >= prob {
					dropItem := ItemTypes[drop.Name]
					w.AddInventoryItem(creature.X, creature.Y, &dropItem)
				}
			}
		}
	}
}

func (w *dbWorld) InventoryItems(x, y uint32) []*InventoryItem {
	items := make([]*InventoryItem, 0)

	pt := Point{X: x, Y: y}
	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, pt.Bytes())
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, pt.Bytes())
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()

	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placeitems"))

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

func (w *dbWorld) AddInventoryItem(x, y uint32, item *InventoryItem) bool {
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

	pt := Point{X: x, Y: y}
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, pt.Bytes())
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)
	dataBytes, err := json.Marshal(inventoryItem)

	if err != nil {
		return false
	}

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placeitems"))

		return bucket.Put(keyBuf.Bytes(), dataBytes)
	})

	return true
}

func (w *dbWorld) inventoryItem(x, y uint32, id string, pull bool) *InventoryItem {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return nil
	}

	idBytes, err := itemID.MarshalBinary()
	if err != nil {
		return nil
	}

	pt := Point{X: x, Y: y}
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, pt.Bytes())
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)

	found := false
	var inventoryItem InventoryItem
	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placeitems"))

		itemBytes := bucket.Get(keyBuf.Bytes())
		if itemBytes != nil {
			if json.Unmarshal(itemBytes, &inventoryItem) == nil {
				inventoryItem.ID = id
				found = true
			}
		}

		if pull && found {
			return bucket.Delete(keyBuf.Bytes())
		}

		return nil
	})

	if found {
		return &inventoryItem
	}
	return nil
}

func (w *dbWorld) InventoryItem(x, y uint32, id string) *InventoryItem {
	return w.inventoryItem(x, y, id, false)
}

func (w *dbWorld) PullInventoryItem(x, y uint32, id string) *InventoryItem {
	return w.inventoryItem(x, y, id, true)
}

func (w *dbWorld) HasInventoryItems(x, y uint32) bool {
	var hasItems bool

	pt := Point{X: x, Y: y}
	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, pt.Bytes())
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, pt.Bytes())
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()

	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placeitems"))

		cur := bucket.Cursor()

		for k, _ := cur.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, _ = cur.Next() {
			hasItems = true

			return nil
		}

		return nil
	})

	return hasItems
}

func (w *dbWorld) NewPlaceID() uint64 {
	id, _ := newPlaceNameInDB(w.database)
	return id
}

func (w *dbWorld) OnlineUsers() []User {
	names := make([]string, 0)
	arr := make([]User, 0)
	offlineNames := make([]string, 0)

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("onlineusers"))
		now := time.Now().UTC().Unix()

		bucket.ForEach(func(k, v []byte) error {
			var lastUpdate int64
			buf := bytes.NewBuffer(v)
			binary.Read(buf, binary.BigEndian, &lastUpdate)

			if (now - lastUpdate) < 15 {
				names = append(names, string(k))
			} else {
				offlineNames = append(offlineNames, string(k))
			}

			return nil
		})

		for _, name := range offlineNames {
			bucket.Delete([]byte(name))
		}

		return nil
	})

	for _, name := range names {
		arr = append(arr, w.GetUser(name))
	}

	for _, name := range offlineNames {
		log.Printf("%s has signed off", name)
		w.Chat(LogItem{Message: fmt.Sprintf("%s has signed off", name), MessageType: MESSAGESYSTEM})
	}

	return arr
}

func (w *dbWorld) usersInCell(p Point) []User {
	arr := make([]User, 0)

	for _, user := range w.OnlineUsers() {
		if *(user.Location()) == p {
			arr = append(arr, user)
		}
	}

	return arr
}

func (w *dbWorld) Chat(message LogItem) {
	for _, user := range w.OnlineUsers() {
		if message.Location == nil || *(message.Location) == *(user.Location()) {
			user.Log(message)
		}
	}
}

func (w *dbWorld) Close() {
	w.closeActiveCells <- struct{}{}
	if w.database != nil {
		w.database.Close()
	}
}

func (w *dbWorld) tickOnActiveItems() {
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-w.closeActiveCells:
			return
		case <-tick:
			w.chargeUsers()
			w.sweepExpiredKeys()
			w.updateActivatedCells()
		}
	}
}

func (w *dbWorld) load() {
	log.Printf("Loading world database %s", w.filename)
	db, err := bolt.Open(w.filename, 0600, nil)

	if err != nil {
		panic(err)
	}

	// Make default tables
	db.Update(func(tx *bolt.Tx) error {
		buckets := []string{"users", "userinventory", "userlog", "onlineusers", "lastuseraction", "terrain", "placenames", "placeitems", "creaturelist", "creatures"}

		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))

			if err != nil {
				return err
			}
		}

		return nil
	})

	w.database = db
	w.closeActiveCells = make(chan struct{})
	go w.tickOnActiveItems()
}

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	newWorld.load()
	return &newWorld
}
