package mud

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Message types for log items
const (
	MESSAGESYSTEM = iota
	MESSAGECHAT
	MESSAGEACTION
)

// LogItem is individual chat log line
type LogItem struct {
	Message     string    `json:""`
	Author      string    `json:""`
	Timestamp   time.Time `json:""`
	MessageType int
}

// Point represents an (X,Y) pair in the world
type Point struct {
	X uint32
	Y uint32
}

// Bytes Dumps a point into a byte array
func (p *Point) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, p)
	return buf.Bytes()
}

// PointFromBytes rehydrates a point struct
func PointFromBytes(ptBytes []byte) Point {
	var pt Point
	buf := bytes.NewBuffer(ptBytes)
	binary.Read(buf, binary.LittleEndian, &pt)
	return pt
}

// LoadResources loads data for the game
func LoadResources() {
	loadCreatureTypes("./bestiary.json")
	loadTerrainTypes("./terrain.json")
}

// MakeTransitionFunction helps build Markov chains.
func MakeTransitionFunction(name string, transitionList []string) (func() string, []string) {
	total := 0

	type transitionName struct {
		name   string
		weight int
	}

	transitionInternalList := make([]transitionName, 0)
	returnTransitionList := make([]string, 0)

	for _, transition := range transitionList {
		splitString := strings.SplitN(transition, ":", 2)
		weightString := "1"
		returnTransitionList = append(returnTransitionList, splitString[0])

		if (len(splitString)) > 1 {
			weightString = splitString[1]
		}

		weight, err := strconv.Atoi(weightString)

		if err != nil {
			panic(err)
		}

		transitionInternalList = append(transitionInternalList, transitionName{name: splitString[0], weight: weight})
		total += weight
	}

	return func() string {
		if transitionInternalList != nil && len(transitionInternalList) != 0 {
			weight := 0
			countTo := rand.Int() % total

			for _, item := range transitionInternalList {
				weight += item.weight

				if weight > countTo {
					return item.name
				}
			}
		}
		return ""
	}, returnTransitionList
}
