package mud

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

// ClassInfo handles user/NPC class orientation
type ClassInfo interface {
	ClassInfo() byte
	SetClassInfo(byte)

	Strengths() (byte, byte)
	SetStrengths(byte, byte)
	Skills() (byte, byte)
	SetSkills(byte, byte)
}
