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
	ClearCreatures(uint32, uint32)
	AddStockCreature(uint32, uint32, string)
	KillCreature(*Point, string)
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
	x, y      uint32
	lastVisit int64
	cellInfo  *CellInfo
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
		AP:         10,
		MaxAP:      10,
		MP:         10,
		MaxMP:      10,
		RP:         10,
		MaxRP:      10,
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
	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{x, y}
		bytes := cellInfoToBytes(cellInfo)
		err := bucket.Put(pt.Bytes(), bytes)

		return err
	})

	ct, ok := CellTypes[cellInfo.TerrainID]

	var spawns []MonsterSpawn

	if ok {
		spawns = ct.MonsterSpawns
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

	return creature
}

func (w *dbWorld) GetCreatures(x, y uint32) []*Creature {
	cl := w.creatureList(x, y)
	creatures := make([]*Creature, 0)

	if cl != nil && len(cl) > 0 {
		for _, id := range cl {
			c := w.getCreature(id)
			if c != nil {
				creatures = append(creatures, c)
			}
		}
	}

	return creatures
}

func (w *dbWorld) HasCreatures(x, y uint32) bool {
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

func (w *dbWorld) ClearCreatures(x, y uint32) {
	creatures := w.creatureList(x, y)

	pt := Point{X: x, Y: y}

	if creatures != nil {
		for _, id := range creatures {
			w.KillCreature(&pt, id)
		}
	}
}

func (w *dbWorld) AddStockCreature(x, y uint32, id string) {
	cID := uuid.New()
	creatureType := CreatureTypes[id]
	creature := &Creature{
		ID:           cID.String(),
		CreatureType: creatureType.ID,
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

func (w *dbWorld) KillCreature(location *Point, id string) {
	cID, err := uuid.Parse(id)

	if err != nil {
		return
	}

	byteID, err := cID.MarshalBinary()

	if err != nil {
		return
	}

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("creatures"))
		err := bucket.Delete(byteID)

		if err != nil {
			return err
		}

		if location != nil {
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
		}

		return nil
	})
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

func (w *dbWorld) Chat(message LogItem) {
	for _, user := range w.OnlineUsers() {
		user.Log(message)
	}
}

func (w *dbWorld) Close() {
	w.closeActiveCells <- struct{}{}
	if w.database != nil {
		w.database.Close()
	}
}

func (w *dbWorld) watchActiveCells() {
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-w.closeActiveCells:
			return
		case <-tick:
			return
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
		buckets := []string{"users", "terrain", "placenames", "userlog", "onlineusers", "creaturelist", "creatures"}

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
	go w.watchActiveCells()
}

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	newWorld.load()
	return &newWorld
}
