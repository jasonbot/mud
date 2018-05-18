package mud

import (
	"fmt"
)

var strengthClassNameMap map[byte]string
var skillClassNameMap map[byte]string

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
	CUNNINGSECONDARY  = byte(16)
	ORDERLYSECONDARY  = byte(32)
	CREATIVESECONDARY = byte(48)
	CUNNINGPRIMARY    = byte(64)
	ORDERLYPRIMARY    = byte(128)
	CREATIVEPRIMARY   = byte(192)
)

// Masks for strengths/skills
const (
	SECONDARYSTRENGTHMASK = byte(3)
	PRIMARYSTRENGTHMASK   = byte(12)
	SECONDARYSKILLMASK    = byte(48)
	PRIMARYSKILLMASK      = byte(192)
)

// ClassInfo handles user/NPC class orientation
type ClassInfo interface {
	ClassInfo() byte
	SetClassInfo(byte)

	Strengths() (byte, byte)
	SetStrengths(byte, byte)
	Skills() (byte, byte)
	SetSkills(byte, byte)
}

// GetTitle takes strengths and gives a class title
func GetTitle(strengthPrimary, strengthSecondary, skillPrimary, skillSecondary byte) string {
	stName := "Neat"
	skName := "Tidy"

	strS, strOK := strengthClassNameMap[strengthPrimary|strengthSecondary]
	if strOK {
		stName = strS
	}

	sklS, sklOK := skillClassNameMap[skillPrimary|skillSecondary]
	if sklOK {
		skName = sklS
	}

	return fmt.Sprintf("%v/%v", stName, skName)
}

func init() {
	strengthClassNameMap = map[byte]string{
		MELEEPRIMARY | MELEESECONDARY: "Warrior",
		MELEEPRIMARY | MAGICSECONDARY: "Paladin",
		MAGICPRIMARY | MELEESECONDARY: "Cleric",
		MAGICPRIMARY | MAGICSECONDARY: "Magician",
		MAGICPRIMARY | RANGESECONDARY: "Mage",
		RANGEPRIMARY | MAGICSECONDARY: "Caster",
		RANGEPRIMARY | RANGESECONDARY: "Sniper",
		RANGEPRIMARY | MELEESECONDARY: "Bower",
		MELEEPRIMARY | RANGESECONDARY: "Ranger"}

	skillClassNameMap = map[byte]string{
		CREATIVEPRIMARY | CREATIVESECONDARY: "Artist",
		CREATIVEPRIMARY | CUNNINGSECONDARY:  "Performer",
		CUNNINGPRIMARY | CREATIVESECONDARY:  "Diplomat",
		CUNNINGPRIMARY | CUNNINGSECONDARY:   "Minister",
		CUNNINGPRIMARY | ORDERLYSECONDARY:   "Counsellor",
		ORDERLYPRIMARY | CUNNINGSECONDARY:   "Demolitionist",
		ORDERLYPRIMARY | ORDERLYSECONDARY:   "Scholar",
		ORDERLYPRIMARY | CREATIVESECONDARY:  "Engineer",
		CREATIVEPRIMARY | ORDERLYSECONDARY:  "Tinkerer"}
}
