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

	bolt "github.com/coreos/bbolt"
	"github.com/google/uuid"
)

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

	cell := w.Cell(x, y)

	rci := &recentCellInfo{
		x:                     x,
		y:                     y,
		lastVisit:             now,
		cellInfo:              cell.CellInfo(),
		creatures:             cell.GetCreatures(),
		lastCreatureAction:    make(map[string]int64),
		desiredCreatureCharge: make(map[string]int64)}

	ci, _ := w.activeCellCache.LoadOrStore(key, rci)
	cellInfo, ok := ci.(*recentCellInfo)

	if ok {
		cellInfo.lastVisit = now
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

		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				cell := w.Cell(uint32(int(userData.X)+x), uint32(int(userData.Y)+y))
				if x == 0 && y == 0 {
					cellData.TerrainID = DefaultCellType
				} else {
					cellData.TerrainID = CellTypes[DefaultCellType].GetRandomTransition()
				}
				cell.SetCellInfo(cellData)
			}
		}
	}

	return userData
}

func (w *dbWorld) Cell(x, y uint32) Cell {
	return &dbCell{w: w, x: x, y: y}
}

func (w *dbWorld) GetCellInfo(x, y uint32) *CellInfo {
	var cellInfo CellInfo
	w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{x, y}
		record := bucket.Get(pt.Bytes())

		if record != nil {
			cellInfo = CellInfoFromBytes(record)
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

func (w *dbWorld) reloadStoredCreatures(x, y uint32) {
	pt := Point{X: x, Y: y}
	record, ok := w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast {
			cell := w.Cell(x, y)
			ci.creatures = cell.GetCreatures()
		}
	}
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

			c := w.Cell(creature.X, creature.Y)
			c.UpdateCreature(creature)

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

type dbCell struct {
	w *dbWorld
	x uint32
	y uint32
}

func (c *dbCell) Location() Point {
	return Point{
		X: c.x,
		Y: c.y}
}

func (c *dbCell) CellInfo() *CellInfo {
	var cellInfo CellInfo
	c.w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{X: c.x, Y: c.y}
		record := bucket.Get(pt.Bytes())

		if record != nil {
			cellInfo = CellInfoFromBytes(record)
		}

		return nil
	})

	placeName := getPlaceNameByIDFromDB(cellInfo.RegionNameID, c.w.database)
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

func (c *dbCell) SetCellInfo(cellInfo *CellInfo) {
	pt := Point{X: c.x, Y: c.y}
	key := pt.Bytes()

	c.w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		bytes := CellInfoToBytes(cellInfo)
		err := bucket.Put(key, bytes)

		return err
	})

	_, ok := c.w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		c.w.activeCellCache.Store(string(key), &recentCellInfo{x: c.x, y: c.y, lastVisit: time.Now().Unix(), cellInfo: cellInfo})
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

					c.AddStockCreature(spawn.Name)
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
					c.AddInventoryItem(&dropItem)
				}
			}
		}
	}
}

func (c *dbCell) reloadStoredCreatures() {
	pt := Point{X: c.x, Y: c.y}
	record, ok := c.w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast {
			ci.creatures = c.getCreatures()
		}
	}
}

func (c *dbCell) creatureList() []string {
	var cl CreatureList

	c.w.database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("creaturelist"))

		pt := Point{X: c.x, Y: c.y}

		bytes := bucket.Get(pt.Bytes())

		if bytes != nil {
			json.Unmarshal(bytes, &cl)
		}

		return nil
	})

	return cl.CreatureIDs
}

func (c *dbCell) getCreatures() []*Creature {
	cl := c.creatureList()
	creatures := make([]*Creature, 0)

	nameFixers := make(map[string]int)

	if cl != nil && len(cl) > 0 {
		for _, id := range cl {
			cr := c.w.getCreature(id)
			if cr != nil {
				_, gotName := nameFixers[cr.CreatureTypeStruct.Name]
				if gotName {
					nameFixers[cr.CreatureTypeStruct.Name] = nameFixers[cr.CreatureTypeStruct.Name] + 1
					cr.CreatureTypeStruct.Name = fmt.Sprintf("%v (%v)", cr.CreatureTypeStruct.Name, nameFixers[cr.CreatureTypeStruct.Name])
				} else {
					nameFixers[cr.CreatureTypeStruct.Name] = 1
				}

				for _, attack := range cr.CreatureTypeStruct.Attacks {
					if attack.Charge > cr.maxCharge {
						cr.maxCharge = attack.Charge
					}
				}

				creatures = append(creatures, cr)
			}
		}
	}

	return creatures
}

func (c *dbCell) GetCreatures() []*Creature {
	cl := c.creatureList()
	creatures := make([]*Creature, 0)

	nameFixers := make(map[string]int)

	if cl != nil && len(cl) > 0 {
		for _, id := range cl {
			c := c.w.getCreature(id)
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

func (c *dbCell) HasCreatures() bool {
	pt := Point{X: c.x, Y: c.y}
	record, ok := c.w.activeCellCache.Load(string(pt.Bytes()))

	if ok {
		ci, cast := record.(*recentCellInfo)

		if cast && ci.creatures != nil {
			if len(ci.creatures) > 0 {
				return true
			}

			return false
		}
	}

	cl := c.creatureList()

	if cl == nil || len(cl) == 0 {
		return false
	}

	for _, id := range cl {
		c := c.w.getCreature(id)
		if c != nil {
			return true
		}
	}

	return false
}

func (c *dbCell) UpdateCreature(creature *Creature) {
	cID, err := uuid.Parse(creature.ID)

	if err != nil {
		return
	}

	c.w.database.Update(func(tx *bolt.Tx) error {
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

	c.w.reloadStoredCreatures(creature.X, creature.Y)
}

func (c *dbCell) ClearCreatures() {
	creatures := c.creatureList()

	if creatures != nil {
		for _, id := range creatures {
			c.w.KillCreature(id)
		}
	}

	c.w.reloadStoredCreatures(c.x, c.y)
}

func (c *dbCell) AddStockCreature(id string) {
	cID := uuid.New()
	creatureType := CreatureTypes[id]
	creature := &Creature{
		ID:           cID.String(),
		CreatureType: creatureType.ID,
		X:            c.x,
		Y:            c.y,
		HP:           creatureType.MaxHP,
		AP:           creatureType.MaxAP,
		MP:           creatureType.MaxMP,
		RP:           creatureType.MaxRP,
		world:        c.w}

	creatureList := CreatureList{}

	c.w.database.Update(func(tx *bolt.Tx) error {
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

		pt := Point{X: c.x, Y: c.y}
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

func (c *dbCell) InventoryItems() []*InventoryItem {
	items := make([]*InventoryItem, 0)

	pt := Point{X: c.x, Y: c.y}
	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, pt.Bytes())
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, pt.Bytes())
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()

	c.w.database.View(func(tx *bolt.Tx) error {
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

func (c *dbCell) AddInventoryItem(item *InventoryItem) bool {
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

	pt := Point{X: c.x, Y: c.y}
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, pt.Bytes())
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)
	dataBytes, err := json.Marshal(inventoryItem)

	if err != nil {
		return false
	}

	c.w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placeitems"))

		return bucket.Put(keyBuf.Bytes(), dataBytes)
	})

	return true
}

func (c *dbCell) inventoryItem(id string, pull bool) *InventoryItem {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return nil
	}

	idBytes, err := itemID.MarshalBinary()
	if err != nil {
		return nil
	}

	pt := Point{X: c.x, Y: c.y}
	keyBuf := new(bytes.Buffer)
	binary.Write(keyBuf, binary.BigEndian, pt.Bytes())
	binary.Write(keyBuf, binary.BigEndian, byte(0))
	binary.Write(keyBuf, binary.BigEndian, idBytes)

	found := false
	var inventoryItem InventoryItem
	c.w.database.Update(func(tx *bolt.Tx) error {
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

func (c *dbCell) InventoryItem(id string) *InventoryItem {
	return c.inventoryItem(id, false)
}

func (c *dbCell) PullInventoryItem(id string) *InventoryItem {
	return c.inventoryItem(id, true)
}

func (c *dbCell) HasInventoryItems() bool {
	var hasItems bool

	pt := Point{X: c.x, Y: c.y}
	minBuf := new(bytes.Buffer)
	maxBuf := new(bytes.Buffer)
	binary.Write(minBuf, binary.BigEndian, pt.Bytes())
	binary.Write(minBuf, binary.BigEndian, byte(0))
	binary.Write(maxBuf, binary.BigEndian, pt.Bytes())
	binary.Write(maxBuf, binary.BigEndian, byte(1))

	min := minBuf.Bytes()
	max := maxBuf.Bytes()

	c.w.database.View(func(tx *bolt.Tx) error {
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

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	newWorld.load()
	return &newWorld
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

func newPlaceNameInDB(db *bolt.DB) (uint64, string) {
	var id uint64
	placeName := randomPlaceName()

	db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placenames"))

		var err error
		id, err = bucket.NextSequence()

		if err != nil {
			return err
		}

		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(id))
		err = bucket.Put(b, []byte(placeName))

		return err
	})

	return id, placeName
}

func getPlaceNameByIDFromDB(id uint64, db *bolt.DB) string {
	placeName := "Delaware"

	db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placenames"))
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(id))
		record := bucket.Get(b)

		if record != nil {
			placeName = string(record)
		}

		return nil
	})

	return placeName
}
