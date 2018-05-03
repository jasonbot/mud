package mud

import (
	"fmt"
	"io"

	"github.com/ahmetb/go-cursor"
	"github.com/mgutz/ansi"
)

// Screen represents a UI screen. For now, just a terminal.
type Screen interface {
}

type sshScreen struct {
}

// NewSSHScreen manages the window rendering for a game session
func NewSSHScreen(terminal io.Writer, world World, user User) Screen {
	screen := sshScreen{}

	return &screen
}

func internalCursorDemo() string {
	clear := cursor.ClearEntireScreen()
	move := cursor.MoveTo(0, 0)
	color := ansi.ColorCode("red+b")
	reset := ansi.ColorCode("reset")

	return fmt.Sprintf("%s%s%sRed%s\n", clear, move, color, reset)
}
