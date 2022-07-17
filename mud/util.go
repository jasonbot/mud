package mud

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/vmihailenco/msgpack"
)

// MessageType is a log message line type
type MessageType int

// Message types for log items
const (
	MESSAGESYSTEM MessageType = iota
	MESSAGECHAT
	MESSAGEACTION
	MESSAGEACTIVITY
)

// LogItem is individual chat log line
type LogItem struct {
	Message     string      `json:""`
	Author      string      `json:""`
	Timestamp   time.Time   `json:""`
	MessageType MessageType `json:""`
	Location    *Point      `json:",omit"`
}

// Point represents an (X,Y) pair in the world
type Point struct {
	X uint32
	Y uint32
}

// Neighbor returns a box in that direction
func (p *Point) Neighbor(d Direction) Point {
	return p.Add(VectorForDirection[d])
}

// Add applies a vector to a point
func (p *Point) Add(v Vector) Point {
	return Point{
		X: uint32(int(p.X) + v.X),
		Y: uint32(int(p.Y) + v.Y)}
}

// Vector Gets the vector between two points such that v = p.Vector(q); p.Add(v) == q
func (p *Point) Vector(v Point) Vector {
	return Vector{
		X: int(v.X) - int(p.X),
		Y: int(v.Y) - int(p.Y)}
}

// Bresenham uses Bresenham's algorithm to visit every involved frame
func (p *Point) Bresenham(v Point, visitor func(Point) error) {
	x0, y0 := p.X, p.Y
	x1, y1 := v.X, v.Y

	if x1 < x0 {
		x0, y0, x1, y1 = x1, y1, x0, y0
	}

	if x0 == x1 { // Vertical line
		if y0 > y1 {
			y0, y1 = y1, y0
		}

		for y := y0; y <= y1; y++ {
			if visitor(Point{X: x0, Y: y}) != nil {
				return
			}
		}

		return
	} else if y0 == y1 { // Horizontal line
		if x0 > x1 {
			x0, x1 = x1, x0
		}

		for x := x0; x <= x1; x++ {
			if visitor(Point{X: x, Y: y0}) != nil {
				return
			}
		}

		return
	}

	deltax := x1 - x0
	deltay := y1 - y0
	deltaerr := math.Abs(float64(deltay) / float64(deltax))
	err := float64(0.0)
	y := y0

	signDeltaY := int(1)
	if math.Signbit(float64(deltay)) {
		signDeltaY = -1
	}

	for x := x0; x <= x1; x++ {
		if visitor(Point{X: uint32(x), Y: uint32(y)}) != nil {
			return
		}

		err += deltaerr
		for err >= 0.5 {
			y = uint32(int(y) + signDeltaY)

			err -= 1.0
		}
	}
}

// Vector is for doing point-to-point comparisons
type Vector struct {
	X int
	Y int
}

// Add combines two vectors
func (v *Vector) Add(p Vector) Vector {
	return Vector{
		X: v.X + p.X,
		Y: v.Y + p.Y}
}

// Magnitude returns the pythagorean theorem to a vector
func (v *Vector) Magnitude() uint {
	return uint(math.Sqrt(math.Pow(float64(v.X), 2.0) + math.Pow(float64(v.Y), 2.0)))
}

// ToBytes flushes point to buffer
func (p *Point) ToBytes(buf io.Writer) {
	binary.Write(buf, binary.LittleEndian, p)
}

// Bytes dumps a point into a byte array
func (p *Point) Bytes() []byte {
	buf := new(bytes.Buffer)
	p.ToBytes(buf)
	return buf.Bytes()
}

// PointFromBytes rehydrates a point struct
func PointFromBytes(ptBytes []byte) Point {
	return PointFromBuffer(bytes.NewBuffer(ptBytes))
}

// PointFromBuffer pulls a point from a byte stream
func PointFromBuffer(buf io.Reader) Point {
	var pt Point
	binary.Read(buf, binary.LittleEndian, &pt)
	return pt
}

// Box represents a Box, ya dingus
type Box struct {
	TopLeft     Point
	BottomRight Point
}

// BoxFromCoords returns a box from coordinates
func BoxFromCoords(x1, y1, x2, y2 uint32) Box {
	if x2 < x1 {
		x1, x2 = x2, x1
	}

	if y2 < y1 {
		y1, y2 = y2, y1
	}

	return Box{Point{x1, y1}, Point{x2, y2}}
}

// BoxFromCenteraAndWidthAndHeight takes a centroid and dimensions
func BoxFromCenteraAndWidthAndHeight(center *Point, width, height uint32) Box {
	topLeft := Point{center.X - width/2, center.Y - height/2}
	return Box{topLeft, Point{topLeft.X + width, topLeft.Y + height}}
}

// WidthAndHeight returns a width, height tuple
func (b *Box) WidthAndHeight() (uint32, uint32) {
	return (b.BottomRight.X - b.TopLeft.X) + 1, (b.BottomRight.Y - b.TopLeft.Y) + 1
}

// ContainsPoint checks point membership
func (b *Box) ContainsPoint(p *Point) bool {
	if p.X >= b.TopLeft.X && p.X <= b.BottomRight.X && p.Y >= b.TopLeft.Y && p.Y <= b.BottomRight.Y {
		return true
	}

	return false
}

// Corners return the corners of a box
func (b *Box) Corners() (Point, Point, Point, Point) {
	return b.TopLeft, Point{b.BottomRight.X, b.TopLeft.Y}, b.BottomRight, Point{b.TopLeft.X, b.BottomRight.Y}
}

// Neighbor returns a box in that direction
func (b *Box) Neighbor(d Direction) Box {
	width, height := b.WidthAndHeight()

	switch d {
	case DIRECTIONNORTH:
		return Box{Point{b.TopLeft.X, b.TopLeft.Y - height}, Point{b.BottomRight.X, b.BottomRight.Y - height}}
	case DIRECTIONEAST:
		return Box{Point{b.TopLeft.X + width, b.TopLeft.Y}, Point{b.BottomRight.X + width, b.BottomRight.Y}}
	case DIRECTIONSOUTH:
		return Box{Point{b.TopLeft.X, b.TopLeft.Y + height}, Point{b.BottomRight.X, b.BottomRight.Y + height}}
	case DIRECTIONWEST:
		return Box{Point{b.TopLeft.X - width, b.TopLeft.Y}, Point{b.BottomRight.X - width, b.BottomRight.Y}}
	}

	return *b
}

// Center returns a point on the middle of the edge, useful for doors
func (b *Box) Center() Point {
	width, height := b.WidthAndHeight()

	return Point{b.TopLeft.X + width/2, b.TopLeft.Y + height/2}
}

// Door returns a point on the middle of the edge, useful for doors
func (b *Box) Door(d Direction) Point {
	width, height := b.WidthAndHeight()

	switch d {
	case DIRECTIONNORTH:
		return Point{b.TopLeft.X + width/2, b.TopLeft.Y}
	case DIRECTIONEAST:
		return Point{b.BottomRight.X, b.TopLeft.Y + height/2}
	case DIRECTIONSOUTH:
		return Point{b.TopLeft.X + width/2, b.BottomRight.Y}
	case DIRECTIONWEST:
		return Point{b.TopLeft.X, b.TopLeft.Y + height/2}
	}

	return b.Center()
}

// Coordinates returns x1 y1 x2 y2
func (b *Box) Coordinates() (uint32, uint32, uint32, uint32) {
	return b.TopLeft.X, b.TopLeft.Y, b.BottomRight.X, b.BottomRight.Y
}

// Direction is a cardinal direction
type Direction byte

// Cardinal directions
const (
	DIRECTIONNORTH Direction = iota
	DIRECTIONEAST
	DIRECTIONSOUTH
	DIRECTIONWEST
)

// ToTheRight gives the direction to the right of the current one
func ToTheRight(d Direction) Direction {
	switch d {
	case DIRECTIONNORTH:
		return DIRECTIONEAST
	case DIRECTIONEAST:
		return DIRECTIONSOUTH
	case DIRECTIONSOUTH:
		return DIRECTIONEAST
	case DIRECTIONWEST:
		return DIRECTIONNORTH
	}

	return DIRECTIONNORTH
}

// ToTheLeft gives the direction to the rigleftt of the current one
func ToTheLeft(d Direction) Direction {
	switch d {
	case DIRECTIONNORTH:
		return DIRECTIONWEST
	case DIRECTIONWEST:
		return DIRECTIONSOUTH
	case DIRECTIONSOUTH:
		return DIRECTIONEAST
	case DIRECTIONEAST:
		return DIRECTIONNORTH
	}

	return DIRECTIONNORTH
}

// VectorForDirection maps directions to a distance vector
var VectorForDirection map[Direction]Vector

// DirectionForVector maps vectors to directions
var DirectionForVector map[Vector]Direction

// LoadResources loads data for the game
func LoadResources() {
	loadCreatureTypes("./bestiary.json")
	loadItemTypes("./items.json")
	loadTerrainTypes("./terrain.json")
}

type transitionName struct {
	name   string
	weight int
}

func makeTransitionGradient(transitionList []string) ([]transitionName, int, []string) {
	total := 0

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

	return transitionInternalList, total, returnTransitionList
}

// MakeGradientTransitionFunction helps build Markov chains.
func MakeGradientTransitionFunction(transitionList []string) func(float64) string {
	transitionInternalList, total, _ := makeTransitionGradient(transitionList)

	return func(inNumber float64) string {
		endWeight := float64(total) * inNumber
		weight := float64(0)

		for _, item := range transitionInternalList {
			weight += float64(item.weight)

			if weight > endWeight {
				return item.name
			}
		}

		return transitionInternalList[len(transitionInternalList)-1].name
	}
}

// MakeTransitionFunction helps build Markov chains.
func MakeTransitionFunction(name string, transitionList []string) (func() string, []string) {

	transitionInternalList, total, returnTransitionList := makeTransitionGradient(transitionList)

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

// MSGPack packs to msgpack using JSON rules
func MSGPack(target interface{}) ([]byte, error) {
	var outBuffer bytes.Buffer

	writer := msgpack.NewEncoder(&outBuffer)
	writer.UseJSONTag(true)
	err := writer.Encode(target)

	return outBuffer.Bytes(), err
}

// MSGUnpack unpacks from msgpack using JSON rules
func MSGUnpack(inBytes []byte, outItem interface{}) error {
	var inBuffer = bytes.NewBuffer(inBytes)

	reader := msgpack.NewDecoder(inBuffer)
	reader.UseJSONTag(true)
	err := reader.Decode(outItem)

	return err
}

func init() {
	VectorForDirection = map[Direction]Vector{
		DIRECTIONNORTH: Vector{X: 0, Y: -1},
		DIRECTIONEAST:  Vector{X: 1, Y: 0},
		DIRECTIONSOUTH: Vector{X: 0, Y: 1},
		DIRECTIONWEST:  Vector{X: -1, Y: 0}}
	DirectionForVector = make(map[Vector]Direction)
	for k, v := range VectorForDirection {
		DirectionForVector[v] = k
	}
}
